package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
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
}

type market struct {
	marketRequest
	Name           string
	Currency       string
	CurrencySymbol string
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

// ===== Yahoo Finance API rate limit & timeout config (defaults, overridable via env vars) =====
// YAHOO_MARKETS_CONCURRENCY: max concurrent requests, default 3, valid range [1, 10]
const yahooFinanceDefaultConcurrency = 3

// YAHOO_MARKETS_JITTER_MS: max random jitter delay before each request in ms, default 400, set to 0 to disable
const yahooFinanceDefaultJitterMs = 400

// YAHOO_MARKETS_TIMEOUT_MS: per-request timeout in ms, default 15000, valid range [3000, 120000]
const yahooFinanceDefaultTimeoutMs = 15000

// yahooFinanceMaxRetries retry count for transient network errors only (EOF / reset / pipe)
// total attempts = 1 (initial) + this value; timeout / 429 / HTTP status errors are NEVER retried
const yahooFinanceMaxRetries = 2

// yahooFinanceRetryBaseDelayMs base backoff delay, multiplied by 2^(retryIndex) for exponential step
const yahooFinanceRetryBaseDelayMs = 600

// yahooFinanceHTTPClient dedicated HTTP client for Yahoo Finance API,
// independent timeout settings to avoid polluting shared defaultHTTPClient
var yahooFinanceHTTPClient = &http.Client{
	Timeout: time.Duration(yahooFinanceDefaultTimeoutMs) * time.Millisecond,
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 10,
		Proxy:               http.ProxyFromEnvironment,
	},
}

// yahooFinanceConcurrency effective concurrency at runtime, read once from env at startup
var yahooFinanceConcurrency = yahooFinanceReadConcurrency()

// yahooFinanceJitter effective jitter delay at runtime, read once from env at startup
var yahooFinanceJitter = yahooFinanceReadJitter()

// yahooFinanceReadTimeout executed as init side-effect, must be declared after yahooFinanceHTTPClient
var _ = yahooFinanceReadTimeout()

func yahooFinanceReadConcurrency() int {
	raw := os.Getenv("YAHOO_MARKETS_CONCURRENCY")
	if raw == "" {
		return yahooFinanceDefaultConcurrency
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 || v > 10 {
		slog.Warn("Invalid YAHOO_MARKETS_CONCURRENCY, fallback to default", "raw", raw, "default", yahooFinanceDefaultConcurrency)
		return yahooFinanceDefaultConcurrency
	}
	return v
}

func yahooFinanceReadJitter() time.Duration {
	raw := os.Getenv("YAHOO_MARKETS_JITTER_MS")
	if raw == "" {
		return time.Duration(yahooFinanceDefaultJitterMs) * time.Millisecond
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		slog.Warn("Invalid YAHOO_MARKETS_JITTER_MS, fallback to default", "raw", raw, "default", yahooFinanceDefaultJitterMs)
		return time.Duration(yahooFinanceDefaultJitterMs) * time.Millisecond
	}
	return time.Duration(v) * time.Millisecond
}

// yahooFinanceReadTimeout applies timeout env override to yahooFinanceHTTPClient,
// must be called after yahooFinanceHTTPClient is declared (package var init order)
func yahooFinanceReadTimeout() bool {
	raw := os.Getenv("YAHOO_MARKETS_TIMEOUT_MS")
	if raw == "" {
		return false
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 3000 || v > 120000 {
		slog.Warn("Invalid YAHOO_MARKETS_TIMEOUT_MS, fallback to default", "raw", raw, "default", yahooFinanceDefaultTimeoutMs)
		return false
	}
	yahooFinanceHTTPClient.Timeout = time.Duration(v) * time.Millisecond
	return true
}

// setYahooFinanceRequestHeaders sets browser-like headers for Yahoo Finance API
func setYahooFinanceRequestHeaders(request *http.Request) {
	request.Header.Set("User-Agent", getBrowserUserAgentHeader())
	request.Header.Set("Accept", "application/json,text/javascript,*/*;q=0.01")
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("Referer", "https://finance.yahoo.com/")
	request.Header.Set("Origin", "https://finance.yahoo.com")
	request.Header.Set("Connection", "keep-alive")
	request.Header.Set("Sec-Fetch-Dest", "empty")
	request.Header.Set("Sec-Fetch-Mode", "cors")
	request.Header.Set("Sec-Fetch-Site", "same-site")
}

// isTransientYahooFinanceError reports whether err is a transient network error worth retrying.
// Scope: only EOF / connection reset / broken pipe / TLS handshake glitches.
// Excluded (intentionally no retry): context deadline (timeout), HTTP 429 / 4xx / 5xx status codes,
// JSON unmarshal errors, symbol / auth errors (400/401/403/404).
func isTransientYahooFinanceError(err error) bool {
	if err == nil {
		return false
	}
	// context deadline / cancellation -> never retry (decision 1-A)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "tls: handshake failure") ||
		strings.Contains(errStr, "tls: bad record MAC") ||
		strings.Contains(errStr, "remote error: tls: handshake failure")
}

// symbolFromRequest extracts the chart symbol from Yahoo Finance request URL path segment for logging.
// Yahoo chart API pattern: /v8/finance/chart/{SYMBOL}; symbol is NOT in query string.
func symbolFromRequest(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	for i, part := range parts {
		if part == "chart" && i+1 < len(parts) {
			sym, _ := url.PathUnescape(parts[i+1])
			return sym
		}
	}
	return ""
}

func fetchMarketsDataFromYahoo(marketRequests []marketRequest) (marketList, error) {
	requests := make([]*http.Request, 0, len(marketRequests))

	for i := range marketRequests {
		request, _ := http.NewRequest("GET", fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1mo&interval=1d", marketRequests[i].Symbol), nil)
		setYahooFinanceRequestHeaders(request)
		requests = append(requests, request)
	}

	// wrap decode task: initial jitter + retry transient-only errors with exponential backoff (2 retries, decision 2-A)
	wrappedTask := func(req *http.Request) (resp marketResponseJson, err error) {
		for attempt := 0; attempt <= yahooFinanceMaxRetries; attempt++ {
			if attempt == 0 {
				if yahooFinanceJitter > 0 {
					time.Sleep(time.Duration(rand.Int64N(int64(yahooFinanceJitter))))
				}
			} else {
				backoff := time.Duration(yahooFinanceRetryBaseDelayMs*(1<<uint(attempt-1))) * time.Millisecond
				backoff += time.Duration(rand.Int64N(int64(backoff/2) + 1))
				time.Sleep(backoff)
				slog.Debug("Retrying Yahoo Finance request after transient error", "symbol", symbolFromRequest(req), "attempt", attempt+1, "prev", err)
			}

			resp, err = decodeJsonFromRequest[marketResponseJson](yahooFinanceHTTPClient, req)
			if err == nil {
				return resp, nil
			}
			if !isTransientYahooFinanceError(err) {
				return resp, err
			}
		}
		return resp, err
	}

	job := newJob(wrappedTask, requests).withWorkers(yahooFinanceConcurrency)
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

		markets = append(markets, market{
			marketRequest:  marketRequests[i],
			Price:          result.Meta.RegularMarketPrice,
			CurrencySymbol: currencyToSymbol[result.Meta.Currency],
			Currency:       result.Meta.Currency,
			PriceHint:      result.Meta.PriceHint,
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
