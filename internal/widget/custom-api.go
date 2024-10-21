package widget

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type CustomApi struct {
	widgetBase       `yaml:",inline"`
	URL              string                       `yaml:"url"`
	Template         string                       `yaml:"template"`
	Frameless        bool                         `yaml:"frameless"`
	Headers          map[string]OptionalEnvString `yaml:"headers"`
	APIRequest       *http.Request                `yaml:"-"`
	compiledTemplate *template.Template           `yaml:"-"`
	CompiledHTML     template.HTML                `yaml:"-"`
}

func (widget *CustomApi) Initialize() error {
	widget.withTitle("Custom API").withCacheDuration(1 * time.Hour)

	if widget.URL == "" {
		return errors.New("URL is required for the custom API widget")
	}

	if widget.Template == "" {
		return errors.New("template is required for the custom API widget")
	}

	compiledTemplate, err := template.New("").Funcs(feed.CustomAPITemplateFuncs).Parse(widget.Template)

	if err != nil {
		return fmt.Errorf("failed parsing custom API widget template: %w", err)
	}

	widget.compiledTemplate = compiledTemplate

	req, err := http.NewRequest(http.MethodGet, widget.URL, nil)
	if err != nil {
		return err
	}

	for key, value := range widget.Headers {
		req.Header.Add(key, value.String())
	}

	widget.APIRequest = req

	return nil
}

func (widget *CustomApi) Update(ctx context.Context) {
	compiledHTML, err := feed.FetchAndParseCustomAPI(widget.APIRequest, widget.compiledTemplate)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.CompiledHTML = compiledHTML
}

func (widget *CustomApi) Render() template.HTML {
	return widget.render(widget, assets.CustomAPITemplate)
}
