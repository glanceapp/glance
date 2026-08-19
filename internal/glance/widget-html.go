package glance

import (
	"html/template"
)

type htmlWidget struct {
	widgetBase `yaml:",inline"`
	Source     template.HTML `yaml:"source"`
}

func (widget *htmlWidget) initialize() error {
	widget.withTitle("").withError(nil)

	return nil
}

func (widget *htmlWidget) Render() template.HTML {
	return widget.Source
}
