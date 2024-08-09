package feed

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

type coingeckoMarketChartResponse struct {
	Prices       [][]float64 `json:"prices"`
	MarketCaps   [][]float64 `json:"market_caps"`
	TotalVolumes [][]float64 `json:"total_volumes"`
}

func fetchSingle(crypto *Cryptocurrency, days int) error {
	if crypto.Days == 0 {
		crypto.Days = days
	}

	if crypto.Currency == "" {
		crypto.Currency = "usd"
	}

	if crypto.SymbolLink == "" {
		crypto.SymbolLink = fmt.Sprintf("https://www.coingecko.com/en/coins/%s", crypto.ID)
	}

	reqURL := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s/market_chart?vs_currency=%s&days=%d", crypto.ID, crypto.Currency, crypto.Days)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}
	// perform request
	resp, err := defaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, reqURL)
	}

	var response coingeckoMarketChartResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return err
	}

	if len(response.Prices) == 0 {
		return fmt.Errorf("no data in response for %s", reqURL)
	}

	crypto.Price = response.Prices[len(response.Prices)-1][1]

	// calculate the percent change
	crypto.PercentChange = percentChange(crypto.Price, response.Prices[0][1])

	var prices []float64
	for _, price := range response.Prices {
		prices = append(prices, price[1])
	}

	crypto.SvgChartPoints = SvgPolylineCoordsFromYValues(100, 50, maybeCopySliceWithoutZeroValues(prices))

	// finally convert the currency into the proper symbol
	crypto.Currency = currencyCodeToSymbol(crypto.Currency)
	return nil
}

func FetchCryptoDataFromCoinGecko(cryptos Cryptocurrencies, days int) (Cryptocurrencies, error) {
	// truncate down to 30 cryptos
	// this is to prevent the rate limit from being hit
	// as the free tier of the CoinGecko API only allows 30 requests per minute

	if len(cryptos) > 30 {
		cryptos = cryptos[:30]
	}

	var wg sync.WaitGroup
	wg.Add(len(cryptos))

	for i := range cryptos {
		go func(crypto *Cryptocurrency) {
			defer wg.Done()
			err := fetchSingle(crypto, days)
			if err != nil {
				slog.Error("Failed to fetch crypto data", "error", err)
			}
		}(&cryptos[i])
	}

	wg.Wait()

	return cryptos, nil
}
