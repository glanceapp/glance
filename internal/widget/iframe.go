package widget

import (
	"errors"
	"fmt"
	"html/template"
	"net/url"

	"github.com/glanceapp/glance/internal/assets"
)

type IFrame struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	Source     string        `yaml:"source"`
	Height     int           `yaml:"height"`
}

func (widget *IFrame) Initialize() error {
	widget.withTitle("IFrame").withError(nil)

	if widget.Source == "" {
		return errors.New("missing source for iframe")
	}

	_, err := url.Parse(widget.Source)

	if err != nil {
		return fmt.Errorf("invalid source for iframe: %v", err)
	}

	if widget.Height == 50 {
		widget.Height = 300
	} else if widget.Height < 50 {
		widget.Height = 50
	}

	widget.cachedHTML = widget.render(widget, assets.IFrameTemplate)

	return nil
}

func (widget *IFrame) Render() template.HTML {
	return widget.cachedHTML
}
