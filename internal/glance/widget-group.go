package glance

import (
	"context"
	"errors"
	"html/template"
	"time"
)

var groupWidgetTemplate = mustParseTemplate("group.html", "widget-base.html")

type groupWidget struct {
	widgetBase          `yaml:",inline"`
	containerWidgetBase `yaml:",inline"`
}

func (widget *groupWidget) initialize() error {
	widget.withError(nil)
	widget.HideHeader = true

	for i := range widget.Widgets {
		widget.Widgets[i].setHideHeader(true)

		if widget.Widgets[i].GetType() == "group" {
			return errors.New("nested groups are not supported")
		} else if widget.Widgets[i].GetType() == "split-column" {
			return errors.New("split columns inside of groups are not supported")
		}
	}

	if err := widget.containerWidgetBase._initializeWidgets(); err != nil {
		return err
	}

	return nil
}

func (widget *groupWidget) update(ctx context.Context) {
	widget.containerWidgetBase._update(ctx)
}

func (widget *groupWidget) setProviders(providers *widgetProviders) {
	widget.containerWidgetBase._setProviders(providers)
}

func (widget *groupWidget) requiresUpdate(now *time.Time) bool {
	return widget.containerWidgetBase._requiresUpdate(now)
}

func (widget *groupWidget) Render() template.HTML {
	return widget.renderTemplate(widget, groupWidgetTemplate)
}
