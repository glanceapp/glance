package glance

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

var extensionWidgetTemplate = mustParseTemplate("extension.html", "widget-base.html")

const extensionWidgetDefaultTitle = "Extension"

type extensionWidget struct {
	widgetBase          `yaml:",inline"`
	URL                 string            `yaml:"url"`
	FallbackContentType string            `yaml:"fallback-content-type"`
	Parameters          map[string]string `yaml:"parameters"`
	AllowHtml           bool              `yaml:"allow-potentially-dangerous-html"`
	Extension           extension         `yaml:"-"`
	cachedHTML          template.HTML     `yaml:"-"`
}

func (widget *extensionWidget) initialize() error {
	widget.withTitle(extensionWidgetDefaultTitle).withCacheDuration(time.Minute * 30)

	if widget.URL == "" {
		return errors.New("URL is required")
	}

	if _, err := url.Parse(widget.URL); err != nil {
		return fmt.Errorf("parsing URL: %v", err)
	}

	return nil
}

func (widget *extensionWidget) update(ctx context.Context) {
	extension, err := fetchExtension(extensionRequestOptions{
		URL:                 widget.URL,
		FallbackContentType: widget.FallbackContentType,
		Parameters:          widget.Parameters,
		AllowHtml:           widget.AllowHtml,
	})

	widget.canContinueUpdateAfterHandlingErr(err)

	widget.Extension = extension

	if widget.Title == extensionWidgetDefaultTitle && extension.Title != "" {
		widget.Title = extension.Title
	}

	widget.cachedHTML = widget.renderTemplate(widget, extensionWidgetTemplate)
}

func (widget *extensionWidget) Render() template.HTML {
	return widget.cachedHTML
}

type extensionType int

const (
	extensionContentHTML    extensionType = iota
	extensionContentUnknown               = iota
)

var extensionStringToType = map[string]extensionType{
	"html": extensionContentHTML,
}

const (
	extensionHeaderTitle            = "Widget-Title"
	extensionHeaderContentType      = "Widget-Content-Type"
	extensionHeaderContentFrameless = "Widget-Content-Frameless"
)

type extensionRequestOptions struct {
	URL                 string            `yaml:"url"`
	FallbackContentType string            `yaml:"fallback-content-type"`
	Parameters          map[string]string `yaml:"parameters"`
	AllowHtml           bool              `yaml:"allow-potentially-dangerous-html"`
}

type extension struct {
	Title     string
	Content   template.HTML
	Frameless bool
}

func convertExtensionContent(options extensionRequestOptions, content []byte, contentType extensionType) template.HTML {
	switch contentType {
	case extensionContentHTML:
		if options.AllowHtml {
			return template.HTML(content)
		}

		fallthrough
	default:
		return template.HTML("<pre>" + html.EscapeString(string(content)) + "</pre>")
	}
}

func fetchExtension(options extensionRequestOptions) (extension, error) {
	request, _ := http.NewRequest("GET", options.URL, nil)

	query := url.Values{}

	for key, value := range options.Parameters {
		query.Set(key, value)
	}

	request.URL.RawQuery = query.Encode()

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		slog.Error("Failed fetching extension", "url", options.URL, "error", err)
		return extension{}, fmt.Errorf("%w: request failed: %w", errNoContent, err)
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Error("Failed reading response body of extension", "url", options.URL, "error", err)
		return extension{}, fmt.Errorf("%w: could not read body: %w", errNoContent, err)
	}

	extension := extension{}

	if response.Header.Get(extensionHeaderTitle) == "" {
		extension.Title = "Extension"
	} else {
		extension.Title = response.Header.Get(extensionHeaderTitle)
	}

	contentType, ok := extensionStringToType[response.Header.Get(extensionHeaderContentType)]

	if !ok {
		contentType, ok = extensionStringToType[options.FallbackContentType]

		if !ok {
			contentType = extensionContentUnknown
		}
	}

	if stringToBool(response.Header.Get(extensionHeaderContentFrameless)) {
		extension.Frameless = true
	}

	extension.Content = convertExtensionContent(options, body, contentType)

	return extension, nil
}
