package glance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

var customAPIWidgetTemplate = mustParseTemplate("custom-api.html", "widget-base.html")

type customAPIWidget struct {
	widgetBase       `yaml:",inline"`
	URL              string             `yaml:"url"`
	Template         string             `yaml:"template"`
	Frameless        bool               `yaml:"frameless"`
	Headers          map[string]string  `yaml:"headers"`
	Body             interface{}        `yaml:"body"`
	FormData         map[string]string  `yaml:"form_data"`
	Functions        map[string]apiFunc `yaml:"functions"`
	APIRequest       *http.Request      `yaml:"-"`
	compiledTemplate *template.Template `yaml:"-"`
	CompiledHTML     template.HTML      `yaml:"-"`
}

type apiFunc struct {
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers"`
	Body      interface{}       `yaml:"body"`
	FormData  map[string]string `yaml:"form_data"`
	Cache     string            `yaml:"cache"`
	APIResult *apiResult        `yaml:"-"`
}

type apiResult struct {
	JSON     decoratedGJSONResult
	Response *http.Response
	Updated  time.Time
}

func (widget *customAPIWidget) initialize() error {
	widget.withTitle("Custom API").withCacheDuration(1 * time.Hour)

	if widget.URL == "" {
		return errors.New("URL is required")
	}

	if widget.Template == "" {
		return errors.New("template is required")
	}

	// Initialize function results storage for each defined function
	for name, fn := range widget.Functions {
		if fn.URL == "" {
			return fmt.Errorf("URL is required for function %s", name)
		}

		// Set default cache duration if not specified
		if fn.Cache == "" {
			fn.Cache = "1h"
		}

		// Create empty result container
		fn.APIResult = &apiResult{}
		widget.Functions[name] = fn
	}

	compiledTemplate, err := template.New("").Funcs(customAPITemplateFuncs).Parse(widget.Template)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	widget.compiledTemplate = compiledTemplate

	var req *http.Request
	// var err error

	// Determine if POST request based on Body or FormData presence
	isPostRequest := widget.Body != nil || len(widget.FormData) > 0
	method := http.MethodGet

	if isPostRequest {
		method = http.MethodPost
		contentType := ""

		if widget.Body != nil {
			// JSON body
			var jsonStr []byte
			jsonStr, err = json.Marshal(widget.Body)
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			bodyReader := bytes.NewBuffer(jsonStr)
			contentType = "application/json"
			req, err = http.NewRequest(method, widget.URL, bodyReader)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", contentType)
		} else if len(widget.FormData) > 0 {
			// Form data
			formValues := url.Values{}
			for key, value := range widget.FormData {
				formValues.Add(key, value)
			}
			bodyReader := strings.NewReader(formValues.Encode())
			contentType = "application/x-www-form-urlencoded"
			req, err = http.NewRequest(method, widget.URL, bodyReader)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", contentType)
		}
	} else {
		// GET request
		req, err = http.NewRequest(method, widget.URL, nil)
		if err != nil {
			return err
		}
	}

	// Add headers
	for key, value := range widget.Headers {
		req.Header.Add(key, value)
	}

	widget.APIRequest = req

	return nil
}

func (widget *customAPIWidget) update(ctx context.Context) {
	// First update all function results
	for name, fn := range widget.Functions {
		if fn.APIResult == nil || fn.APIResult.Updated.IsZero() || time.Since(fn.APIResult.Updated) > parseCacheDuration(fn.Cache) {
			result, err := fetchAPIResult(fn)
			if err != nil {
				slog.Error("Error fetching function result", "function", name, "error", err)
				continue
			}
			fn.APIResult = result
			widget.Functions[name] = fn
		}
	}

	// Then update the main widget with function results available in template
	compiledHTML, err := fetchAndParseCustomAPI(widget.APIRequest, widget.compiledTemplate, widget.Functions)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.CompiledHTML = compiledHTML
}

// Parse cache duration string into time.Duration
func parseCacheDuration(cacheDuration string) time.Duration {
	duration, err := time.ParseDuration(cacheDuration)
	if err != nil {
		// Default to 1 hour if invalid
		return 1 * time.Hour
	}
	return duration
}

// Fetch API result for a function
func fetchAPIResult(fn apiFunc) (*apiResult, error) {
	var req *http.Request
	var err error

	// Determine if POST request based on Body or FormData presence
	isPostRequest := fn.Body != nil || len(fn.FormData) > 0
	method := http.MethodGet

	if isPostRequest {
		method = http.MethodPost
		contentType := ""

		if fn.Body != nil {
			// JSON body
			var jsonStr []byte
			jsonData, ok := fn.Body.(map[string]interface{})
			if ok {
				jsonStr, err = json.Marshal(jsonData)
				if err != nil {
					return nil, fmt.Errorf("marshaling JSON: %w", err)
				}
				bodyReader := bytes.NewBuffer(jsonStr)
				contentType = "application/json"
				req, err = http.NewRequest(method, fn.URL, bodyReader)
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", contentType)
			}
		} else if len(fn.FormData) > 0 {
			// Form data
			formValues := url.Values{}
			for key, value := range fn.FormData {
				formValues.Add(key, value)
			}
			bodyReader := strings.NewReader(formValues.Encode())
			contentType = "application/x-www-form-urlencoded"
			req, err = http.NewRequest(method, fn.URL, bodyReader)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", contentType)
		}
	} else {
		// GET request
		req, err = http.NewRequest(method, fn.URL, nil)
		if err != nil {
			return nil, err
		}
	}

	// Add headers
	for key, value := range fn.Headers {
		req.Header.Add(key, value)
	}

	// Fetch result
	emptyResult := &apiResult{}
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return emptyResult, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return emptyResult, err
	}

	body := string(bodyBytes)

	// Check if response is valid JSON
	if !gjson.Valid(body) {
		truncatedBody, isTruncated := limitStringLength(body, 100)
		if isTruncated {
			truncatedBody += "... <truncated>"
		}

		slog.Error("Invalid response JSON in function", "url", req.URL.String(), "body", truncatedBody)
		return emptyResult, errors.New("invalid response JSON")
	}

	return &apiResult{
		JSON:     decoratedGJSONResult{gjson.Parse(body)},
		Response: resp,
		Updated:  time.Now(),
	}, nil
}

func (widget *customAPIWidget) Render() template.HTML {
	return widget.renderTemplate(widget, customAPIWidgetTemplate)
}

func fetchAndParseCustomAPI(req *http.Request, tmpl *template.Template, functions map[string]apiFunc) (template.HTML, error) {
	emptyBody := template.HTML("")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return emptyBody, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return emptyBody, err
	}

	body := string(bodyBytes)

	// Check if response is valid JSON
	if !gjson.Valid(body) {
		truncatedBody, isTruncated := limitStringLength(body, 100)
		if isTruncated {
			truncatedBody += "... <truncated>"
		}

		slog.Error("Invalid response JSON in custom API widget", "url", req.URL.String(), "body", truncatedBody)
		return emptyBody, errors.New("invalid response JSON")
	}

	var templateBuffer bytes.Buffer

	data := customAPITemplateData{
		JSON:      decoratedGJSONResult{gjson.Parse(body)},
		Response:  resp,
		Functions: functions,
	}

	err = tmpl.Execute(&templateBuffer, &data)
	if err != nil {
		return emptyBody, err
	}

	return template.HTML(templateBuffer.String()), nil
}

type decoratedGJSONResult struct {
	gjson.Result
}

type customAPITemplateData struct {
	JSON      decoratedGJSONResult
	Response  *http.Response
	Functions map[string]apiFunc
}

func gJsonResultArrayToDecoratedResultArray(results []gjson.Result) []decoratedGJSONResult {
	decoratedResults := make([]decoratedGJSONResult, len(results))

	for i, result := range results {
		decoratedResults[i] = decoratedGJSONResult{result}
	}

	return decoratedResults
}

func (r *decoratedGJSONResult) Exists(key string) bool {
	return r.Get(key).Exists()
}

func (r *decoratedGJSONResult) Array(key string) []decoratedGJSONResult {
	if key == "" {
		return gJsonResultArrayToDecoratedResultArray(r.Result.Array())
	}

	return gJsonResultArrayToDecoratedResultArray(r.Get(key).Array())
}

func (r *decoratedGJSONResult) String(key string) string {
	if key == "" {
		return r.Result.String()
	}

	return r.Get(key).String()
}

func (r *decoratedGJSONResult) Int(key string) int {
	if key == "" {
		return int(r.Result.Int())
	}

	return int(r.Get(key).Int())
}

func (r *decoratedGJSONResult) Float(key string) float64 {
	if key == "" {
		return r.Result.Float()
	}

	return r.Get(key).Float()
}

func (r *decoratedGJSONResult) Bool(key string) bool {
	if key == "" {
		return r.Result.Bool()
	}

	return r.Get(key).Bool()
}

var customAPITemplateFuncs = func() template.FuncMap {
	funcs := template.FuncMap{
		"toFloat": func(a int) float64 {
			return float64(a)
		},
		"toInt": func(a float64) int {
			return int(a)
		},
		"add": func(a, b float64) float64 {
			return a + b
		},
		"sub": func(a, b float64) float64 {
			return a - b
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"div": func(a, b float64) float64 {
			if b == 0 {
				return math.NaN()
			}

			return a / b
		},
		// Integer-specific math functions to avoid type conversion
		"addInt": func(a, b int) int {
			return a + b
		},
		"subInt": func(a, b int) int {
			return a - b
		},
		"mulInt": func(a, b int) int {
			return a * b
		},
		"divInt": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"funcString": func(data customAPITemplateData, funcName, path string) string {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return ""
			}
			return fn.APIResult.JSON.String(path)
		},
		"funcInt": func(data customAPITemplateData, funcName, path string) int {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return 0
			}
			return fn.APIResult.JSON.Int(path)
		},
		"funcFloat": func(data customAPITemplateData, funcName, path string) float64 {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return 0
			}
			return fn.APIResult.JSON.Float(path)
		},
		"funcBool": func(data customAPITemplateData, funcName, path string) bool {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return false
			}
			return fn.APIResult.JSON.Bool(path)
		},
		"funcExists": func(data customAPITemplateData, funcName, path string) bool {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return false
			}
			return fn.APIResult.JSON.Exists(path)
		},
		"funcArray": func(data customAPITemplateData, funcName, path string) []decoratedGJSONResult {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return []decoratedGJSONResult{}
			}
			return fn.APIResult.JSON.Array(path)
		},
		// New helper functions for looping over arrays
		"arrayLen": func(data customAPITemplateData, funcName, path string) int {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return 0
			}
			return len(fn.APIResult.JSON.Array(path))
		},
		"arrayItem": func(data customAPITemplateData, funcName, path string, index int) decoratedGJSONResult {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return decoratedGJSONResult{}
			}
			arr := fn.APIResult.JSON.Array(path)
			if index < 0 || index >= len(arr) {
				return decoratedGJSONResult{}
			}
			return arr[index]
		},
		// For filtering array items based on conditions
		"filter": func(data customAPITemplateData, funcName, path, filterKey, filterValue string) []decoratedGJSONResult {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return []decoratedGJSONResult{}
			}
			
			arr := fn.APIResult.JSON.Array(path)
			result := make([]decoratedGJSONResult, 0)
			
			for _, item := range arr {
				if item.String(filterKey) == filterValue {
					result = append(result, item)
				}
			}
			
			return result
		},
		// Helper to create a range of integers for loop iterations
		"intRange": func(start, end int) []int {
			if end < start {
				return []int{}
			}
			
			result := make([]int, end-start+1)
			for i := range result {
				result[i] = start + i
			}
			return result
		},
		// Sort array items by a specific field
		"sortBy": func(data customAPITemplateData, funcName, path, sortField string) []decoratedGJSONResult {
			fn, exists := data.Functions[funcName]
			if !exists || fn.APIResult == nil {
				return []decoratedGJSONResult{}
			}
			
			arr := fn.APIResult.JSON.Array(path)
			// Since we can't dynamically sort in Go templates, we'll return the original array
			// Sorting would need to be implemented in a preprocessing step before template execution
			return arr
		},
	}

	for key, value := range globalTemplateFunctions {
		if _, exists := funcs[key]; !exists {
			funcs[key] = value
		}
	}

	return funcs
}()
