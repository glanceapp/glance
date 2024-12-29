package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strings"
	"time"
)

var twitchGamesWidgetTemplate = mustParseTemplate("twitch-games-list.html", "widget-base.html")

type twitchGamesWidget struct {
	widgetBase    `yaml:",inline"`
	Categories    []twitchCategory `yaml:"-"`
	Exclude       []string         `yaml:"exclude"`
	Limit         int              `yaml:"limit"`
	CollapseAfter int              `yaml:"collapse-after"`
}

func (widget *twitchGamesWidget) initialize() error {
	widget.
		withTitle("Top games on Twitch").
		withTitleURL("https://www.twitch.tv/directory?sort=VIEWER_COUNT").
		withCacheDuration(time.Minute * 10)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *twitchGamesWidget) update(ctx context.Context) {
	categories, err := fetchTopGamesFromTwitch(widget.Exclude, widget.Limit)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Categories = categories
}

func (widget *twitchGamesWidget) Render() template.HTML {
	return widget.renderTemplate(widget, twitchGamesWidgetTemplate)
}

type twitchCategory struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	AvatarUrl    string `json:"avatarURL"`
	ViewersCount int    `json:"viewersCount"`
	Tags         []struct {
		Name string `json:"tagName"`
	} `json:"tags"`
	GameReleaseDate string `json:"originalReleaseDate"`
	IsNew           bool   `json:"-"`
}

type twitchDirectoriesOperationResponse struct {
	Data struct {
		DirectoriesWithTags struct {
			Edges []struct {
				Node twitchCategory `json:"node"`
			} `json:"edges"`
		} `json:"directoriesWithTags"`
	} `json:"data"`
}

const twitchDirectoriesOperationRequestBody = `[
{"operationName": "BrowsePage_AllDirectories","variables": {"limit": %d,"options": {"sort": "VIEWER_COUNT","tags": []}},"extensions": {"persistedQuery": {"version": 1,"sha256Hash": "2f67f71ba89f3c0ed26a141ec00da1defecb2303595f5cda4298169549783d9e"}}}
]`

func fetchTopGamesFromTwitch(exclude []string, limit int) ([]twitchCategory, error) {
	reader := strings.NewReader(fmt.Sprintf(twitchDirectoriesOperationRequestBody, len(exclude)+limit))
	request, _ := http.NewRequest("POST", twitchGqlEndpoint, reader)
	request.Header.Add("Client-ID", twitchGqlClientId)
	response, err := decodeJsonFromRequest[[]twitchDirectoriesOperationResponse](defaultHTTPClient, request)
	if err != nil {
		return nil, err
	}

	if len(response) == 0 {
		return nil, errors.New("no categories could be retrieved")
	}

	edges := (response)[0].Data.DirectoriesWithTags.Edges
	categories := make([]twitchCategory, 0, len(edges))

	for i := range edges {
		if slices.Contains(exclude, edges[i].Node.Slug) {
			continue
		}

		category := &edges[i].Node
		category.AvatarUrl = strings.Replace(category.AvatarUrl, "285x380", "144x192", 1)

		if len(category.Tags) > 2 {
			category.Tags = category.Tags[:2]
		}

		gameReleasedDate, err := time.Parse("2006-01-02T15:04:05Z", category.GameReleaseDate)

		if err == nil {
			if time.Since(gameReleasedDate) < 14*24*time.Hour {
				category.IsNew = true
			}
		}

		categories = append(categories, *category)
	}

	if len(categories) > limit {
		categories = categories[:limit]
	}

	return categories, nil
}
