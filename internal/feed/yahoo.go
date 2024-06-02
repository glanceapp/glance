package feed

import (
	"fmt"
	"log/slog"
	"net/http"
)

type stockResponseJson struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
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
const stockChartDays = 21

func FetchStocksDataFromYahoo(stockRequests Stocks) (Stocks, error) {
	requests := make([]*http.Request, 0, len(stockRequests))

	for i := range stockRequests {
		request, _ := http.NewRequest("GET", fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1mo&interval=1d", stockRequests[i].Symbol), nil)
		requests = append(requests, request)
	}

	job := newJob(decodeJsonFromRequestTask[stockResponseJson](defaultClient), requests)
	responses, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}

	stocks := make(Stocks, 0, len(responses))
	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch stock data", "symbol", stockRequests[i].Symbol, "error", errs[i])
			continue
		}

		response := responses[i]

		if len(response.Chart.Result) == 0 {
			failed++
			slog.Error("Stock response contains no data", "symbol", stockRequests[i].Symbol)
			continue
		}

		prices := response.Chart.Result[0].Indicators.Quote[0].Close

		if len(prices) > stockChartDays {
			prices = prices[len(prices)-stockChartDays:]
		}

		previous := response.Chart.Result[0].Meta.RegularMarketPrice

		if len(prices) >= 2 && prices[len(prices)-2] != 0 {
			previous = prices[len(prices)-2]
		}

		points := SvgPolylineCoordsFromYValues(100, 50, maybeCopySliceWithoutZeroValues(prices))

		currency, exists := currencyToSymbol[response.Chart.Result[0].Meta.Currency]

		if !exists {
			currency = response.Chart.Result[0].Meta.Currency
		}

		stocks = append(stocks, Stock{
			Name:       stockRequests[i].Name,
			Symbol:     response.Chart.Result[0].Meta.Symbol,
			SymbolLink: stockRequests[i].SymbolLink,
			ChartLink:  stockRequests[i].ChartLink,
			Price:      response.Chart.Result[0].Meta.RegularMarketPrice,
			Currency:   currency,
			PercentChange: percentChange(
				response.Chart.Result[0].Meta.RegularMarketPrice,
				previous,
			),
			SvgChartPoints: points,
		})
	}

	if len(stocks) == 0 {
		return nil, ErrNoContent
	}

	if failed > 0 {
		return stocks, fmt.Errorf("%w: could not fetch data for %d stock(s)", ErrPartialContent, failed)
	}

	return stocks, nil
}
