package feed

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/tidwall/gjson"
)

func FetchAndParseCustomAPI(req *http.Request, tmpl *template.Template) (template.HTML, error) {
	emptyBody := template.HTML("")

	resp, err := defaultClient.Do(req)
	if err != nil {
		return emptyBody, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return emptyBody, err
	}

	body := string(bodyBytes)

	if !gjson.Valid(body) {
		truncatedBody, isTruncated := limitStringLength(body, 100)
		if isTruncated {
			truncatedBody += "... <truncated>"
		}

		slog.Error("invalid response JSON in custom API widget", "URL", req.URL.String(), "body", truncatedBody)
		return emptyBody, errors.New("invalid response JSON")
	}

	var templateBuffer bytes.Buffer

	data := CustomAPITemplateData{
		JSON:     DecoratedGJSONResult{gjson.Parse(body)},
		Response: resp,
	}

	err = tmpl.Execute(&templateBuffer, &data)
	if err != nil {
		return emptyBody, err
	}

	return template.HTML(templateBuffer.String()), nil
}

type DecoratedGJSONResult struct {
	gjson.Result
}

type CustomAPITemplateData struct {
	JSON     DecoratedGJSONResult
	Response *http.Response
}

func GJsonResultArrayToDecoratedResultArray(results []gjson.Result) []DecoratedGJSONResult {
	decoratedResults := make([]DecoratedGJSONResult, len(results))

	for i, result := range results {
		decoratedResults[i] = DecoratedGJSONResult{result}
	}

	return decoratedResults
}

func (r *DecoratedGJSONResult) Array(key string) []DecoratedGJSONResult {
	if key == "" {
		return GJsonResultArrayToDecoratedResultArray(r.Result.Array())
	}

	return GJsonResultArrayToDecoratedResultArray(r.Get(key).Array())
}

func (r *DecoratedGJSONResult) String(key string) string {
	if key == "" {
		return r.Result.String()
	}

	return r.Get(key).String()
}

func (r *DecoratedGJSONResult) Int(key string) int64 {
	if key == "" {
		return r.Result.Int()
	}

	return r.Get(key).Int()
}

func (r *DecoratedGJSONResult) Float(key string) float64 {
	if key == "" {
		return r.Result.Float()
	}

	return r.Get(key).Float()
}

func (r *DecoratedGJSONResult) Bool(key string) bool {
	if key == "" {
		return r.Result.Bool()
	}

	return r.Get(key).Bool()
}

var CustomAPITemplateFuncs = func() template.FuncMap {
	funcs := template.FuncMap{
		"toFloat": func(a int64) float64 {
			return float64(a)
		},
		"toInt": func(a float64) int64 {
			return int64(a)
		},
		"mathexpr": func(left float64, op string, right float64) float64 {
			if right == 0 {
				return 0
			}

			switch op {
			case "+":
				return left + right
			case "-":
				return left - right
			case "*":
				return left * right
			case "/":
				return left / right
			default:
				return 0
			}
		},
	}

	for key, value := range assets.GlobalTemplateFunctions {
		funcs[key] = value
	}

	return funcs
}()
