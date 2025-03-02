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
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

var customAPIWidgetTemplate = mustParseTemplate("custom-api.html", "widget-base.html")
var customRandomKeyForSingleRequest = fmt.Sprintf("%x", time.Now().UnixNano())

type customAPIWidget struct {
	widgetBase       `yaml:",inline"`
	ApiQueries		 map[string]apiQueries		`yaml:"api-queries"`
	URL              string               		`yaml:"url"`
	Template         string               		`yaml:"template"`
	Frameless        bool                 		`yaml:"frameless"`
	Headers          map[string]string    		`yaml:"headers"`
	Parameters       queryParametersField 		`yaml:"parameters"`
	APIRequest       map[string]*http.Request	`yaml:"-"`
	compiledTemplate *template.Template   		`yaml:"-"`
	CompiledHTML     template.HTML        		`yaml:"-"`
}

type apiQueries struct {
	URL              string               `yaml:"url"`
	Headers          map[string]string    `yaml:"headers"`
	Parameters       queryParametersField `yaml:"parameters"`
}

func (widget *customAPIWidget) initialize() error {
	widget.withTitle("Custom API").withCacheDuration(1 * time.Hour)

	widget.APIRequest = make(map[string]*http.Request)
	if len(widget.ApiQueries) != 0 {
		for object, query := range widget.ApiQueries {
			if query.URL == "" {
				return errors.New("URL for each query is required")
			}
			req, err := http.NewRequest(http.MethodGet, query.URL, nil)
			if err != nil {
				return err
			}

			req.URL.RawQuery = query.Parameters.toQueryString()

			for key, value := range query.Headers {
				req.Header.Add(key, value)
			}

			widget.APIRequest[object] = req
		}
	} else {
		if widget.URL == "" {
			return errors.New("URL is required")
		}
		
		req, err := http.NewRequest(http.MethodGet, widget.URL, nil)
		if err != nil {
			return err
		}
		
		req.URL.RawQuery = widget.Parameters.toQueryString()
		
		for key, value := range widget.Headers {
			req.Header.Add(key, value)
		}
		
		widget.APIRequest[customRandomKeyForSingleRequest] = req
	}

	if widget.Template == "" {
		return errors.New("template is required")
	}

	compiledTemplate, err := template.New("").Funcs(customAPITemplateFuncs).Parse(widget.Template)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	widget.compiledTemplate = compiledTemplate

	return nil
}

func (widget *customAPIWidget) update(ctx context.Context) {
	compiledHTML, err := fetchAndParseCustomAPI(widget.APIRequest, widget.compiledTemplate)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.CompiledHTML = compiledHTML
}

func (widget *customAPIWidget) Render() template.HTML {
	return widget.renderTemplate(widget, customAPIWidgetTemplate)
}

func fetchAndParseCustomAPI(requests map[string]*http.Request, tmpl *template.Template) (template.HTML, error) {
	emptyBody := template.HTML("")

	var resp *http.Response
	var err error
	body := make(map[string]string)
	for key, req := range requests {
		resp, err = defaultHTTPClient.Do(req)
		if err != nil {
			return emptyBody, err
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return emptyBody, err
		}
	
		body[key] = strings.TrimSpace(string(bodyBytes))
	
		if body[key] != "" && !gjson.Valid(body[key]) {
			truncatedBody, isTruncated := limitStringLength(body[key], 100)
			if isTruncated {
				truncatedBody += "... <truncated>"
			}
	
			slog.Error("Invalid response JSON in custom API widget", "url", req.URL.String(), key, truncatedBody)
			return emptyBody, errors.New("invalid response JSON")
		}
	}
	
	mergedBody := "{}"
	if jsonBody, exists := body[customRandomKeyForSingleRequest]; exists {
		mergedBody = jsonBody
	} else {
		mergedMap := make(map[string]json.RawMessage)
		for key, jsonBody := range body {
			if !gjson.Valid(jsonBody) {
				continue
			}
			mergedMap[key] = json.RawMessage(jsonBody)
		}
		if len(mergedMap) > 0 {
			bytes, _ := json.Marshal(mergedMap)
			mergedBody = string(bytes)
		}
	}

	var templateBuffer bytes.Buffer

	data := customAPITemplateData{
		JSON:     decoratedGJSONResult{gjson.Parse(mergedBody)},
		Response: resp,
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
	JSON     decoratedGJSONResult
	Response *http.Response
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

func (r *decoratedGJSONResult) Int(key string) int64 {
	if key == "" {
		return r.Result.Int()
	}

	return r.Get(key).Int()
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
		"toFloat": func(a int64) float64 {
			return float64(a)
		},
		"toInt": func(a float64) int64 {
			return int64(a)
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
	}

	for key, value := range globalTemplateFunctions {
		if _, exists := funcs[key]; !exists {
			funcs[key] = value
		}
	}

	return funcs
}()
