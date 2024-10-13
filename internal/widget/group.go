package widget

import (
	"context"
	"errors"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
)

type Group struct {
	widgetBase      `yaml:",inline"`
	containerWidget `yaml:",inline"`
}

func (widget *Group) Initialize() error {
	widget.withError(nil)
	widget.HideHeader = true

	for i := range widget.Widgets {
		widget.Widgets[i].SetHideHeader(true)

		if widget.Widgets[i].GetType() == "group" {
			return errors.New("nested groups are not allowed")
		}

		if err := widget.Widgets[i].Initialize(); err != nil {
			return err
		}
	}

	return nil
}

func (widget *Group) Update(ctx context.Context) {
	widget.containerWidget.Update(ctx)
}

func (widget *Group) SetProviders(providers *Providers) {
	widget.containerWidget.SetProviders(providers)
}

func (widget *Group) RequiresUpdate(now *time.Time) bool {
	return widget.containerWidget.RequiresUpdate(now)
}

func (widget *Group) Render() template.HTML {
	return widget.render(widget, assets.GroupTemplate)
}
