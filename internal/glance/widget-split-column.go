package glance

import (
	"context"
	"html/template"
	"time"
)

var splitColumnWidgetTemplate = mustParseTemplate("split-column.html", "widget-base.html")

type splitColumnWidget struct {
	widgetBase          `yaml:",inline"`
	containerWidgetBase `yaml:",inline"`
	MaxColumns          int `yaml:"max-columns"`
}

func (widget *splitColumnWidget) initialize() error {
	widget.withError(nil).withTitle("Split Column").setHideHeader(true)

	if err := widget.containerWidgetBase._initializeWidgets(); err != nil {
		return err
	}

	if widget.MaxColumns < 2 {
		widget.MaxColumns = 2
	}

	return nil
}

func (widget *splitColumnWidget) update(ctx context.Context) {
	widget.containerWidgetBase._update(ctx)
}

func (widget *splitColumnWidget) setProviders(providers *widgetProviders) {
	widget.containerWidgetBase._setProviders(providers)
}

func (widget *splitColumnWidget) requiresUpdate(now *time.Time) bool {
	return widget.containerWidgetBase._requiresUpdate(now)
}

func (widget *splitColumnWidget) Render() template.HTML {
	return widget.renderTemplate(widget, splitColumnWidgetTemplate)
}
