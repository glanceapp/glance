package widget

import (
	"html/template"
)

type HTML struct {
	widgetBase `yaml:",inline"`
	Source     template.HTML `yaml:"source"`
}

func (widget *HTML) Initialize() error {
	widget.withTitle("").withError(nil)

	return nil
}

func (widget *HTML) Render() template.HTML {
	return widget.Source
}
