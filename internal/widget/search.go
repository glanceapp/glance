package widget

import (
	"html/template"

	"github.com/glanceapp/glance/internal/assets"
)

type Search struct {
	widgetBase `yaml:",inline"`
	SearchURL  string `yaml:"search-url"`
	Query      string `yaml:"query"`
}

func (widget *Search) Initialize() error {
	widget.withTitle("Search").withError(nil)

	if widget.SearchURL == "" {
		// set to the duckduckgo search engine
		widget.SearchURL = "https://duckduckgo.com/?q="
	}

	// if no query is provided, leave an empty string

	return nil
}

func (widget *Search) Render() template.HTML {
	return widget.render(widget, assets.SearchTemplate)
}
