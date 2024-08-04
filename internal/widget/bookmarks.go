package widget

import (
	"context"
	"github.com/glanceapp/glance/internal/feed"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
)

type Bookmarks struct {
	widgetBase `yaml:",inline"`
	Groups     []struct {
		Title string         `yaml:"title"`
		Color *HSLColorField `yaml:"color"`
		Links []struct {
			Title        string           `yaml:"title"`
			URL          string           `yaml:"url"`
			Icon         string           `yaml:"icon"`
			IsSimpleIcon bool             `yaml:"-"`
			SameTab      bool             `yaml:"same-tab"`
			HideArrow    bool             `yaml:"hide-arrow"`
			StatusPage   *feed.StatusPage `yaml:"status-page"`
		} `yaml:"links"`
	} `yaml:"groups"`
	Style string `yaml:"style"`
}

func (widget *Bookmarks) Initialize() error {
	countStatusPages := 0

	for g := range widget.Groups {
		for l := range widget.Groups[g].Links {
			if widget.Groups[g].Links[l].StatusPage != nil {
				countStatusPages++
			}

			if widget.Groups[g].Links[l].Icon == "" {
				continue
			}

			link := &widget.Groups[g].Links[l]
			link.Icon, link.IsSimpleIcon = toSimpleIconIfPrefixed(link.Icon)
		}
	}

	w := widget.withTitle("Bookmarks")
	if countStatusPages > 0 {
		w.withCacheDuration(30 * time.Minute)
	} else {
		w.withError(nil)
	}

	return nil
}

func (widget *Bookmarks) Update(ctx context.Context) {
	countStatusPages := 0
	for g := range widget.Groups {
		for l := range widget.Groups[g].Links {
			if widget.Groups[g].Links[l].StatusPage != nil {
				if widget.Groups[g].Links[l].StatusPage.URL != "" {
					countStatusPages++
				}
			}
		}
	}

	if countStatusPages == 0 {
		return
	}

	requests := make([]*feed.StatusPage, countStatusPages)

	countStatusPages = 0
	for g := range widget.Groups {
		for l := range widget.Groups[g].Links {
			if widget.Groups[g].Links[l].StatusPage != nil {
				if widget.Groups[g].Links[l].StatusPage.URL != "" {
					requests[countStatusPages] = widget.Groups[g].Links[l].StatusPage
					countStatusPages++
				}
			}
		}
	}

	statuses, err := feed.FetchStatusPages(requests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	for g := range widget.Groups {
		for l := range widget.Groups[g].Links {
			if widget.Groups[g].Links[l].StatusPage != nil {
				if widget.Groups[g].Links[l].StatusPage.URL != "" {
					widget.Groups[g].Links[l].StatusPage.StatusPageInfo = statuses[0]
					statuses = statuses[1:]
				}
			}
		}
	}
}

func (widget *Bookmarks) Render() template.HTML {
	return widget.render(widget, assets.BookmarksTemplate)
}
