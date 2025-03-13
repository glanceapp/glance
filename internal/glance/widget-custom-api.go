package glance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

var customAPIWidgetTemplate = mustParseTemplate("custom-api.html", "widget-base.html")

// Needs to be exported for the YAML unmarshaler to work
type CustomAPIRequest struct {
	URL         string               `json:"url"`
	Headers     map[string]string    `json:"headers"`
	Parameters  queryParametersField `json:"parameters"`
	httpRequest *http.Request        `yaml:"-"`
}

type customAPIWidget struct {
	widgetBase        `yaml:",inline"`
	*CustomAPIRequest `yaml:",inline"`             // the primary request
	Subrequests       map[string]*CustomAPIRequest `yaml:"subrequests"`
	Template          string                       `yaml:"template"`
	Frameless         bool                         `yaml:"frameless"`
	compiledTemplate  *template.Template           `yaml:"-"`
	CompiledHTML      template.HTML                `yaml:"-"`
}

func (widget *customAPIWidget) initialize() error {
	widget.withTitle("Custom API").withCacheDuration(1 * time.Hour)

	if err := widget.CustomAPIRequest.initialize(); err != nil {
		return fmt.Errorf("initializing primary request: %v", err)
	}

	for key := range widget.Subrequests {
		if err := widget.Subrequests[key].initialize(); err != nil {
			return fmt.Errorf("initializing subrequest %q: %v", key, err)
		}
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
	compiledHTML, err := fetchAndParseCustomAPI(widget.CustomAPIRequest, widget.Subrequests, widget.compiledTemplate)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.CompiledHTML = compiledHTML
}

func (widget *customAPIWidget) Render() template.HTML {
	return widget.renderTemplate(widget, customAPIWidgetTemplate)
}

func (req *CustomAPIRequest) initialize() error {
	if req.URL == "" {
		return errors.New("URL is required")
	}

	httpReq, err := http.NewRequest(http.MethodGet, req.URL, nil)
	if err != nil {
		return err
	}

	if len(req.Parameters) > 0 {
		httpReq.URL.RawQuery = req.Parameters.toQueryString()
	}

	for key, value := range req.Headers {
		httpReq.Header.Add(key, value)
	}

	req.httpRequest = httpReq

	return nil
}

type customAPIResponseData struct {
	JSON     decoratedGJSONResult
	Response *http.Response
}

type customAPITemplateData struct {
	*customAPIResponseData
	subrequests map[string]*customAPIResponseData
}

func (data *customAPITemplateData) Subrequest(key string) *customAPIResponseData {
	req, exists := data.subrequests[key]
	if !exists {
		// We have to panic here since there's nothing sensible we can return and the
		// lack of an error would cause requested data to return zero values which
		// would be confusing from the user's perspective. Go's template module
		// handles recovering from panics and will return the panic message as an
		// error during template execution.
		panic(fmt.Sprintf("subrequest with key %q has not been defined", key))
	}

	return req
}

func fetchCustomAPIRequest(ctx context.Context, req *CustomAPIRequest) (*customAPIResponseData, error) {
	resp, err := defaultHTTPClient.Do(req.httpRequest.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	body := strings.TrimSpace(string(bodyBytes))

	if body != "" && !gjson.Valid(body) {
		truncatedBody, isTruncated := limitStringLength(body, 100)
		if isTruncated {
			truncatedBody += "... <truncated>"
		}

		slog.Error("Invalid response JSON in custom API widget", "url", req.httpRequest.URL.String(), "body", truncatedBody)
		return nil, errors.New("invalid response JSON")
	}

	data := &customAPIResponseData{
		JSON:     decoratedGJSONResult{gjson.Parse(body)},
		Response: resp,
	}

	return data, nil
}

func fetchAndParseCustomAPI(
	primaryReq *CustomAPIRequest,
	subReqs map[string]*CustomAPIRequest,
	tmpl *template.Template,
) (template.HTML, error) {
	var primaryData *customAPIResponseData
	subData := make(map[string]*customAPIResponseData, len(subReqs))
	var err error

	if len(subReqs) == 0 {
		// If there are no subrequests, we can fetch the primary request in a much simpler way
		primaryData, err = fetchCustomAPIRequest(context.Background(), primaryReq)
	} else {
		// If there are subrequests, we need to fetch them concurrently
		// and cancel all requests if any of them fail. There's probably
		// a more elegant way to do this, but this works for now.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		var mu sync.Mutex // protects subData and err

		wg.Add(1)
		go func() {
			defer wg.Done()
			var localErr error
			primaryData, localErr = fetchCustomAPIRequest(ctx, primaryReq)
			mu.Lock()
			if localErr != nil && err == nil {
				err = localErr
				cancel()
			}
			mu.Unlock()
		}()

		for key, req := range subReqs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var localErr error
				var data *customAPIResponseData
				data, localErr = fetchCustomAPIRequest(ctx, req)
				mu.Lock()
				if localErr == nil {
					subData[key] = data
				} else if err == nil {
					err = localErr
					cancel()
				}
				mu.Unlock()
			}()
		}

		wg.Wait()
	}

	emptyBody := template.HTML("")

	if err != nil {
		return emptyBody, err
	}

	data := customAPITemplateData{
		customAPIResponseData: primaryData,
		subrequests:           subData,
	}

	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, &data)
	if err != nil {
		return emptyBody, err
	}

	return template.HTML(templateBuffer.String()), nil
}

type decoratedGJSONResult struct {
	gjson.Result
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
		"parseTime": func(layout, value string) time.Time {
			switch strings.ToLower(layout) {
			case "rfc3339":
				layout = time.RFC3339
			case "rfc3339nano":
				layout = time.RFC3339Nano
			case "datetime":
				layout = time.DateTime
			case "dateonly":
				layout = time.DateOnly
			case "timeonly":
				layout = time.TimeOnly
			}

			parsed, err := time.Parse(layout, value)
			if err != nil {
				return time.Unix(0, 0)
			}

			return parsed
		},
		"toRelativeTime": dynamicRelativeTimeAttrs,
	}

	for key, value := range globalTemplateFunctions {
		if _, exists := funcs[key]; !exists {
			funcs[key] = value
		}
	}

	return funcs
}()
