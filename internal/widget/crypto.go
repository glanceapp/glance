package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Crypto struct {
	widgetBase `yaml:",inline"`
	Cryptos    feed.Cryptocurrencies `yaml:"cryptos"`
	Sort       string                `yaml:"sort-by"`
	Style      string                `yaml:"style"`
	Days       int                   `yaml:"days"`
}

func (widget *Crypto) Initialize() error {
	widget.withTitle("Crypto").withCacheDuration(time.Hour)

	if widget.Days == 0 {
		widget.Days = 1
	}

	return nil
}

func (widget *Crypto) Update(ctx context.Context) {
	cryptos, err := feed.FetchCryptoDataFromCoinGecko(widget.Cryptos, widget.Days)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.Sort == "percent-change" {
		cryptos.SortByPercentChange()
	}

	widget.Cryptos = cryptos
}

func (widget *Crypto) Render() template.HTML {
	return widget.render(widget, assets.CryptoTemplate)
}
