package glance

import (
	"html/template"
)

var bookmarksWidgetTemplate = mustParseTemplate("bookmarks.html", "widget-base.html")

type bookmarksWidget struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	Groups     []struct {
		Title string         `yaml:"title"`
		Color *hslColorField `yaml:"color"`
		Links []struct {
			Title     string           `yaml:"title"`
			URL       optionalEnvField `yaml:"url"`
			Icon      customIconField  `yaml:"icon"`
			SameTab   bool             `yaml:"same-tab"`
			HideArrow bool             `yaml:"hide-arrow"`
		} `yaml:"links"`
	} `yaml:"groups"`
}

func (widget *bookmarksWidget) initialize() error {
	widget.withTitle("Bookmarks").withError(nil)
	widget.cachedHTML = widget.renderTemplate(widget, bookmarksWidgetTemplate)

	return nil
}

func (widget *bookmarksWidget) Render() template.HTML {
	return widget.cachedHTML
}
