package widget

import (
	"context"
	"html/template"

	"github.com/glanceapp/glance/internal/assets"
)

type Clock struct {
	widgetBase `yaml:",inline"`
}

func (widget *Clock) Initialize() error {
	widget.withTitle("Clock").withError(nil)
	return nil
}

func (widget *Clock) Update(ctx context.Context) {}

func (widget *Clock) Render() template.HTML {
	return widget.render(widget, assets.ClockTemplate)
}
