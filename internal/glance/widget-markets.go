package glance

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

var marketsWidgetTemplate = mustParseTemplate("markets.html", "widget-base.html")

type marketsWidget struct {
	widgetBase         `yaml:",inline"`
	StocksRequests     []marketRequest `yaml:"stocks"`
	MarketRequests     []marketRequest `yaml:"markets"`
	ChartLinkTemplate  string          `yaml:"chart-link-template"`
	SymbolLinkTemplate string          `yaml:"symbol-link-template"`
	Sort               string          `yaml:"sort-by"`
	Markets            marketList      `yaml:"-"`
}

func (widget *marketsWidget) initialize() error {
	widget.withTitle("Markets").withCacheDuration(time.Hour)

	// legacy support, remove in v0.10.0
	if len(widget.MarketRequests) == 0 {
		widget.MarketRequests = widget.StocksRequests
	}

	for i := range widget.MarketRequests {
		m := &widget.MarketRequests[i]

		if widget.ChartLinkTemplate != "" && m.ChartLink == "" {
			m.ChartLink = strings.ReplaceAll(widget.ChartLinkTemplate, "{SYMBOL}", m.Symbol)
		}

		if widget.SymbolLinkTemplate != "" && m.SymbolLink == "" {
			m.SymbolLink = strings.ReplaceAll(widget.SymbolLinkTemplate, "{SYMBOL}", m.Symbol)
		}

		if m.Range == "" {
			m.Range = "1mo"
		} else if !marketValidRanges[m.Range] {
			return fmt.Errorf("invalid range %q for symbol %s, valid values: 1d 5d 1mo 3mo 6mo 1y 2y 5y 10y ytd max", m.Range, m.Symbol)
		}

		if m.Interval == "" {
			m.Interval = "1d"
		} else if !marketValidIntervals[m.Interval] {
			return fmt.Errorf("invalid interval %q for symbol %s, valid values: 1m 2m 5m 15m 30m 60m 90m 1h 4h 1d 5d 1wk 1mo 3mo", m.Interval, m.Symbol)
		}
	}

	return nil
}

func (widget *marketsWidget) update(ctx context.Context) {
	markets, err := fetchMarketsDataFromYahoo(widget.MarketRequests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.Sort == "absolute-change" {
		markets.sortByAbsChange()
	} else if widget.Sort == "change" {
		markets.sortByChange()
	}

	widget.Markets = markets
}

func (widget *marketsWidget) Render() template.HTML {
	return widget.renderTemplate(widget, marketsWidgetTemplate)
}

type marketRequest struct {
	CustomName string `yaml:"name"`
	Symbol     string `yaml:"symbol"`
	ChartLink  string `yaml:"chart-link"`
	SymbolLink string `yaml:"symbol-link"`
	Range      string `yaml:"range"`
	Interval   string `yaml:"interval"`
}

type market struct {
	marketRequest
	Name           string
	Currency       string
	Price          float64
	PriceHint      int
	PercentChange  float64
	SvgChartPoints string
}

type marketList []market

func (t marketList) sortByAbsChange() {
	sort.Slice(t, func(i, j int) bool {
		return math.Abs(t[i].PercentChange) > math.Abs(t[j].PercentChange)
	})
}

func (t marketList) sortByChange() {
	sort.Slice(t, func(i, j int) bool {
		return t[i].PercentChange > t[j].PercentChange
	})
}

type marketResponseJson struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				ExchangeName       string  `json:"exchangeName"`
				ShortName          string  `json:"shortName"`
				PriceHint          int     `json:"priceHint"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close,omitempty"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// TODO: allow changing chart time frame
const marketChartDays = 21

func fetchMarketsDataFromYahoo(marketRequests []marketRequest) (marketList, error) {
	requests := make([]*http.Request, 0, len(marketRequests))

	for i := range marketRequests {
		url := fmt.Sprintf(
			"https://query1.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=%s",
			marketRequests[i].Symbol,
			marketRequests[i].Range,
			marketRequests[i].Interval,
		)
		request, _ := http.NewRequest("GET", url, nil)
		setBrowserUserAgentHeader(request)
		requests = append(requests, request)
	}

	job := newJob(decodeJsonFromRequestTask[marketResponseJson](defaultHTTPClient), requests)
	responses, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	markets := make(marketList, 0, len(responses))
	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch market data", "symbol", marketRequests[i].Symbol, "error", errs[i])
			continue
		}

		response := responses[i]

		if len(response.Chart.Result) == 0 {
			failed++
			slog.Error("Market response contains no data", "symbol", marketRequests[i].Symbol)
			continue
		}

		result := &response.Chart.Result[0]
		prices := result.Indicators.Quote[0].Close

		if len(prices) > marketChartDays {
			prices = prices[len(prices)-marketChartDays:]
		}

		previous := result.Meta.RegularMarketPrice

		if len(prices) >= 2 && prices[len(prices)-2] != 0 {
			previous = prices[len(prices)-2]
		}

		points := svgPolylineCoordsFromYValues(100, 50, maybeCopySliceWithoutZeroValues(prices))

		currency, exists := currencyToSymbol[strings.ToUpper(result.Meta.Currency)]
		if !exists {
			currency = result.Meta.Currency
		}

		// See https://github.com/glanceapp/glance/issues/757
		if result.Meta.ExchangeName == "LSE" {
			currency = ""
		}

		markets = append(markets, market{
			marketRequest: marketRequests[i],
			Price:         result.Meta.RegularMarketPrice,
			Currency:      currency,
			PriceHint:     result.Meta.PriceHint,
			Name: ternary(marketRequests[i].CustomName == "",
				result.Meta.ShortName,
				marketRequests[i].CustomName,
			),
			PercentChange: percentChange(
				result.Meta.RegularMarketPrice,
				previous,
			),
			SvgChartPoints: points,
		})
	}

	if len(markets) == 0 {
		return nil, errNoContent
	}

	if failed > 0 {
		return markets, fmt.Errorf("%w: could not fetch data for %d market(s)", errPartialContent, failed)
	}

	return markets, nil
}

var currencyToSymbol = map[string]string{
	"USD": "$",
	"EUR": "€",
	"JPY": "¥",
	"CAD": "C$",
	"AUD": "A$",
	"GBP": "£",
	"CHF": "Fr",
	"NZD": "N$",
	"INR": "₹",
	"BRL": "R$",
	"RUB": "₽",
	"TRY": "₺",
	"ZAR": "R",
	"CNY": "¥",
	"KRW": "₩",
	"HKD": "HK$",
	"SGD": "S$",
	"SEK": "kr",
	"NOK": "kr",
	"DKK": "kr",
	"PLN": "zł",
	"PHP": "₱",
}

var marketValidRanges = map[string]bool{
	"1d": true, "5d": true, "1mo": true, "3mo": true, "6mo": true,
	"1y": true, "2y": true, "5y": true, "10y": true, "ytd": true, "max": true,
}

var marketValidIntervals = map[string]bool{
	"1m": true, "2m": true, "5m": true, "15m": true, "30m": true,
	"60m": true, "90m": true, "1h": true, "4h": true,
	"1d": true, "5d": true, "1wk": true, "1mo": true, "3mo": true,
}
