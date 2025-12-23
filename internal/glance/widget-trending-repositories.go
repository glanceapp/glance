package glance

import (
	"context"
	"fmt"
	"html/template"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var trendingRepositoriesWidgetTemplate = mustParseTemplate("trending-repositories.html", "widget-base.html")

type trendingRepositoriesWidget struct {
	widgetBase   `yaml:",inline"`
	Repositories []trendingRepository `yaml:"-"`

	Language      string `yaml:"language"`
	DateRange     string `yaml:"date-range"`
	CollapseAfter int    `yaml:"collapse-after"`
}

func (widget *trendingRepositoriesWidget) initialize() error {
	widget.withTitle("Trending Repositories").withCacheDuration(8 * time.Hour)

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 4
	}

	if !slices.Contains([]string{"daily", "weekly", "monthly"}, widget.DateRange) {
		widget.DateRange = "daily"
	}

	return nil
}

func (widget *trendingRepositoriesWidget) update(ctx context.Context) {
	repositories, err := widget.fetchTrendingRepositories()

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Repositories = repositories
}

func (widget *trendingRepositoriesWidget) Render() template.HTML {
	return widget.renderTemplate(widget, trendingRepositoriesWidgetTemplate)
}

type trendingRepository struct {
	Slug        string
	Description string
	Language    string
	Stars       int
}

func (widget *trendingRepositoriesWidget) fetchTrendingRepositories() ([]trendingRepository, error) {
	url := fmt.Sprintf("https://github.com/trending/%s?since=%s", widget.Language, widget.DateRange)

	response, err := defaultHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch trending repositories: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code %d for %s", response.StatusCode, url)
	}

	parsedDoc, err := html.Parse(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML response: %w", err)
	}

	doc := (*searchableNode)(parsedDoc)
	repositories := make([]trendingRepository, 0, 15)

	repoElems := doc.
		findFirst("main").
		findFirst("div", "class", "Box").
		findAll("article", "class", "Box-row")

	for _, repoElem := range repoElems {
		nameElem := repoElem.findFirstChild("h2").findFirst("a", "class", "Link")
		name := strings.ReplaceAll(nameElem.text(), " ", "")

		description := repoElem.findFirstChild("p").text()
		metaElem := repoElem.findFirstChild("div", "class", "f6 color-fg-muted mt-2")

		language := metaElem.findFirst("span", "itemprop", "programmingLanguage").text()
		starsIndex := 2
		if language == "" {
			starsIndex = 1
		}

		starsText := metaElem.nthChild(starsIndex).text()
		starsText = strings.ReplaceAll(starsText, ",", "")
		stars, _ := strconv.Atoi(starsText)

		repositories = append(repositories, trendingRepository{
			Slug:        name,
			Description: description,
			Language:    language,
			Stars:       stars,
		})
	}

	return repositories, nil
}
