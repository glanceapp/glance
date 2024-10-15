package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
)

type SplitColumn struct {
	widgetBase          `yaml:",inline"`
	containerWidgetBase `yaml:",inline"`
}

func (widget *SplitColumn) Initialize() error {
	widget.withError(nil).withTitle("Split Column").SetHideHeader(true)

	for i := range widget.Widgets {
		if err := widget.Widgets[i].Initialize(); err != nil {
			return err
		}
	}

	return nil
}

func (widget *SplitColumn) Update(ctx context.Context) {
	widget.containerWidgetBase.Update(ctx)
}

func (widget *SplitColumn) SetProviders(providers *Providers) {
	widget.containerWidgetBase.SetProviders(providers)
}

func (widget *SplitColumn) RequiresUpdate(now *time.Time) bool {
	return widget.containerWidgetBase.RequiresUpdate(now)
}

func (widget *SplitColumn) Render() template.HTML {
	return widget.render(widget, assets.SplitColumnTemplate)
}
