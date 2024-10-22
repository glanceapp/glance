package widget

import (
	"fmt"
	"html/template"

	"github.com/glanceapp/glance/internal/assets"
)

type HeadingSize int

const (
	Size1 HeadingSize = iota + 1
	Size2
	Size3
	Size4
	Size5
)

type Heading struct {
	widgetBase `yaml:",inline"`
	Size       HeadingSize `yaml:"size"`
	Text       string      `yaml:"text"`
	Separator  bool        `yaml:"separator"`
	Frameless  bool        `yaml:"frameless"`
}

func (widget *Heading) Initialize() error {
	widget.withTitle("").withError(nil)

	if widget.Size < Size1 || widget.Size > Size5 {
		return fmt.Errorf("invalid heading size: %d", widget.Size)
	}

	widget.HideHeader = true
	return nil
}

func (widget *Heading) Render() template.HTML {
	return widget.render(widget, assets.HeadingTemplate)
}
