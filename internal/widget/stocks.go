package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

// TODO: rename to Markets at some point
type Stocks struct {
	widgetBase `yaml:",inline"`
	Stocks     feed.Stocks `yaml:"stocks"`
	Sort       string      `yaml:"sort-by"`
}

func (widget *Stocks) Initialize() error {
	widget.withTitle("Stocks").withCacheDuration(time.Hour)

	return nil
}

func (widget *Stocks) Update(ctx context.Context) {
	stocks, err := feed.FetchStocksDataFromYahoo(widget.Stocks)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.Sort == "absolute-change" {
		stocks.SortByAbsChange()
	}

	widget.Stocks = stocks
}

func (widget *Stocks) Render() template.HTML {
	return widget.render(widget, assets.StocksTemplate)
}
