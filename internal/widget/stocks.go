package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Markets struct {
	widgetBase     `yaml:",inline"`
	StocksRequests []feed.MarketRequest `yaml:"stocks"`
	MarketRequests []feed.MarketRequest `yaml:"markets"`
	Sort           string               `yaml:"sort-by"`
	Style          string               `yaml:"style"`
	Markets        feed.Markets         `yaml:"-"`
}

func (widget *Markets) Initialize() error {
	widget.withTitle("Markets").withCacheDuration(time.Hour)

	if len(widget.MarketRequests) == 0 {
		widget.MarketRequests = widget.StocksRequests
	}

	return nil
}

func (widget *Markets) Update(ctx context.Context) {
	markets, err := feed.FetchMarketsDataFromYahoo(widget.MarketRequests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.Sort == "absolute-change" {
		markets.SortByAbsChange()
	}

	widget.Markets = markets
}

func (widget *Markets) Render() template.HTML {
	return widget.render(widget, assets.MarketsTemplate)
}
