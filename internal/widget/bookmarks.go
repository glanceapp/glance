package widget

import (
	"html/template"

	"github.com/glanceapp/glance/internal/assets"
)

type Bookmarks struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	Groups     []struct {
		Title string         `yaml:"title"`
		Color *HSLColorField `yaml:"color"`
		Links []struct {
			Title     string     `yaml:"title"`
			URL       string     `yaml:"url"`
			Icon      CustomIcon `yaml:"icon"`
			SameTab   bool       `yaml:"same-tab"`
			HideArrow bool       `yaml:"hide-arrow"`
		} `yaml:"links"`
	} `yaml:"groups"`
}

func (widget *Bookmarks) Initialize() error {
	widget.withTitle("Bookmarks").withError(nil)
	widget.cachedHTML = widget.render(widget, assets.BookmarksTemplate)

	return nil
}

func (widget *Bookmarks) Render() template.HTML {
	return widget.cachedHTML
}
