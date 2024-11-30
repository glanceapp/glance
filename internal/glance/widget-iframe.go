package glance

import (
	"errors"
	"fmt"
	"html/template"
	"net/url"
)

var iframeWidgetTemplate = mustParseTemplate("iframe.html", "widget-base.html")

type iframeWidget struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	Source     string        `yaml:"source"`
	Height     int           `yaml:"height"`
}

func (widget *iframeWidget) initialize() error {
	widget.withTitle("IFrame").withError(nil)

	if widget.Source == "" {
		return errors.New("source is required")
	}

	if _, err := url.Parse(widget.Source); err != nil {
		return fmt.Errorf("parsing URL: %v", err)
	}

	if widget.Height == 50 {
		widget.Height = 300
	} else if widget.Height < 50 {
		widget.Height = 50
	}

	widget.cachedHTML = widget.renderTemplate(widget, iframeWidgetTemplate)

	return nil
}

func (widget *iframeWidget) Render() template.HTML {
	return widget.cachedHTML
}
