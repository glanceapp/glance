package widget

import (
	"html/template"
	"strings"

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

			if strings.HasPrefix(widget.Groups[g].Links[l].Icon, "si:") {
				icon := strings.TrimPrefix(widget.Groups[g].Links[l].Icon, "si:")
				widget.Groups[g].Links[l].IsSimpleIcon = true
				widget.Groups[g].Links[l].Icon = "https://cdnjs.cloudflare.com/ajax/libs/simple-icons/11.14.0/" + icon + ".svg"
			}
		}
	}

	widget.cachedHTML = widget.render(widget, assets.BookmarksTemplate)

	return nil
}

func (widget *Bookmarks) Render() template.HTML {
	return widget.cachedHTML
}
