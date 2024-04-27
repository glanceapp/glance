package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Stocks struct {
	widgetBase `yaml:",inline"`
	Stocks     feed.Stocks         `yaml:"-"`
	Tickers    []feed.StockRequest `yaml:"stocks"`
}

func (widget *Stocks) Initialize() error {
	widget.withTitle("Stocks").withCacheDuration(time.Hour)

	return nil
}

func (widget *Stocks) Update(ctx context.Context) {
	stocks, err := feed.FetchStocksDataFromYahoo(widget.Tickers)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	stocks.SortByAbsChange()
	widget.Stocks = stocks
}

func (widget *Stocks) Render() template.HTML {
	return widget.render(widget, assets.StocksTemplate)
}
