package glance

import (
	"context"
	"errors"
	"html/template"
	"time"
)

var stackWidgetTemplate = mustParseTemplate("stack.html", "widget-base.html")

type stackWidget struct {
	widgetBase          `yaml:",inline"`
	containerWidgetBase `yaml:",inline"`
}

func (widget *stackWidget) initialize() error {
	widget.withError(nil)
	widget.HideHeader = true

	for i := range widget.Widgets {

		if widget.Widgets[i].GetType() == "stack" {
			return errors.New("nested stacks are not supported")
		} else if widget.Widgets[i].GetType() == "group" {
			return errors.New("groups inside of stacks are not supported")
		} else if widget.Widgets[i].GetType() == "split-column" {
			return errors.New("split columns inside of stacks are not supported")
		}
	}

	if err := widget.containerWidgetBase._initializeWidgets(); err != nil {
		return err
	}

	return nil
}

func (widget *stackWidget) update(ctx context.Context) {
	widget.containerWidgetBase._update(ctx)
}

func (widget *stackWidget) setProviders(providers *widgetProviders) {
	widget.containerWidgetBase._setProviders(providers)
}

func (widget *stackWidget) requiresUpdate(now *time.Time) bool {
	return widget.containerWidgetBase._requiresUpdate(now)
}

func (widget *stackWidget) Render() template.HTML {
	return widget.renderTemplate(widget, stackWidgetTemplate)
}
