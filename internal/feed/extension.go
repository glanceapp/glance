package feed

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

type ExtensionType int

const (
	ExtensionContentHTML    ExtensionType = iota
	ExtensionContentUnknown               = iota
)

var ExtensionStringToType = map[string]ExtensionType{
	"html": ExtensionContentHTML,
}

const (
	ExtensionHeaderTitle       = "Widget-Title"
	ExtensionHeaderContentType = "Widget-Content-Type"
)

type ExtensionRequestOptions struct {
	URL        string            `yaml:"url"`
	Parameters map[string]string `yaml:"parameters"`
	AllowHtml  bool              `yaml:"allow-potentially-dangerous-html"`
}

type Extension struct {
	Title   string
	Content template.HTML
}

func convertExtensionContent(options ExtensionRequestOptions, content []byte, contentType ExtensionType) template.HTML {
	switch contentType {
	case ExtensionContentHTML:
		if options.AllowHtml {
			return template.HTML(content)
		}

		fallthrough
	default:
		return template.HTML(html.EscapeString(string(content)))
	}
}

func FetchExtension(options ExtensionRequestOptions) (Extension, error) {
	request, _ := http.NewRequest("GET", options.URL, nil)

	query := url.Values{}

	for key, value := range options.Parameters {
		query.Set(key, value)
	}

	request.URL.RawQuery = query.Encode()

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		slog.Error("failed fetching extension", "error", err, "url", options.URL)
		return Extension{}, fmt.Errorf("%w: request failed: %w", ErrNoContent, err)
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		slog.Error("failed reading response body of extension", "error", err, "url", options.URL)
		return Extension{}, fmt.Errorf("%w: could not read body: %w", ErrNoContent, err)
	}

	extension := Extension{}

	if response.Header.Get(ExtensionHeaderTitle) == "" {
		extension.Title = "Extension"
	} else {
		extension.Title = response.Header.Get(ExtensionHeaderTitle)
	}

	contentType, ok := ExtensionStringToType[response.Header.Get(ExtensionHeaderContentType)]

	if !ok {
		contentType = ExtensionContentUnknown
	}

	extension.Content = convertExtensionContent(options, body, contentType)

	return extension, nil
}
