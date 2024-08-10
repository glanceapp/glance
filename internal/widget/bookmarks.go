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
			Title        string `yaml:"title"`
			URL          string `yaml:"url"`
			Icon         string `yaml:"icon"`
			IsSimpleIcon bool   `yaml:"-"`
			SameTab      bool   `yaml:"same-tab"`
			HideArrow    bool   `yaml:"hide-arrow"`
		} `yaml:"links"`
	} `yaml:"groups"`
	Style string `yaml:"style"`
}

func (widget *Bookmarks) Initialize() error {
	widget.withTitle("Bookmarks").withError(nil)

	for g := range widget.Groups {
		for l := range widget.Groups[g].Links {
			if widget.Groups[g].Links[l].Icon == "" {
				continue
			}

			link := &widget.Groups[g].Links[l]
			link.Icon, link.IsSimpleIcon = toSimpleIconIfPrefixed(link.Icon)
		}
	}

	widget.cachedHTML = widget.render(widget, assets.BookmarksTemplate)

	return nil
}

func (widget *Bookmarks) Render() template.HTML {
	return widget.cachedHTML
}
