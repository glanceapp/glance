package glance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"
)

var ghostfolioWidgetTemplate = mustParseTemplate("ghostfolio.html", "widget-base.html")

type ghostfolioWidget struct {
	widgetBase    `yaml:",inline"`
	URL           string `yaml:"url"`
	AccessToken   string `yaml:"access-token"`
	DefaultRange  string `yaml:"range"`
	ChartType     string `yaml:"chart-type"`   // "value" or "performance"
	ChartHeight   int    `yaml:"chart-height"` // Height in pixels (default: 100)
	AllowInsecure bool   `yaml:"allow-insecure"`
	// All portfolio data for different ranges (preloaded)
	PortfolioData map[string]*ghostfolioPortfolio `yaml:"-"`
}

func (widget *ghostfolioWidget) GetChartType() string {
	return widget.ChartType
}

func (widget *ghostfolioWidget) GetChartHeight() int {
	return widget.ChartHeight
}

type ghostfolioPortfolio struct {
	TotalValue             float64                    `json:"totalValue"`
	NetPerformance         float64                    `json:"netPerformance"`
	NetPerformancePct      float64                    `json:"netPerformancePct"`
	Currency               string                     `json:"currency"`
	ChartPoints            string                     `json:"chartPoints"`            // Portfolio value chart
	PerformanceChartPoints string                     `json:"performanceChartPoints"` // Performance % chart
	ChartData              []ghostfolioChartDataPoint `json:"chartData"`              // Raw data for tooltips
	HasData                bool                       `json:"hasData"`
}

// Simplified chart data point for frontend tooltips
type ghostfolioChartDataPoint struct {
	Date           string  `json:"date"`
	Value          float64 `json:"value"`
	PerformancePct float64 `json:"performancePct"`
}

// Available range options for the widget
// Ghostfolio API supports: 1d, wtd, mtd, ytd, max
var ghostfolioRangeOptions = []struct {
	Value string
	Label string
}{
	{"1d", "1D"},
	{"wtd", "WTD"},
	{"mtd", "MTD"},
	{"ytd", "YTD"},
	{"max", "Max"},
}

func (widget *ghostfolioWidget) GetRangeOptions() []struct {
	Value string
	Label string
} {
	return ghostfolioRangeOptions
}

func (widget *ghostfolioWidget) GetDefaultRange() string {
	return widget.DefaultRange
}

func (widget *ghostfolioWidget) GetPortfolio() *ghostfolioPortfolio {
	if widget.PortfolioData == nil {
		return nil
	}
	return widget.PortfolioData[widget.DefaultRange]
}

// GetPortfolioDataJSON returns all preloaded portfolio data as JSON for the frontend
func (widget *ghostfolioWidget) GetPortfolioDataJSON() template.JS {
	if widget.PortfolioData == nil {
		return template.JS("{}")
	}
	data, err := json.Marshal(widget.PortfolioData)
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(data)
}

func (widget *ghostfolioWidget) initialize() error {
	widget.withTitle("Portfolio").withCacheDuration(15 * time.Minute)

	if widget.URL == "" {
		return fmt.Errorf("ghostfolio widget requires a URL")
	}

	if widget.AccessToken == "" {
		return fmt.Errorf("ghostfolio widget requires an access-token")
	}

	// Remove trailing slash from URL
	widget.URL = strings.TrimSuffix(widget.URL, "/")

	// Default range
	if widget.DefaultRange == "" {
		widget.DefaultRange = "max"
	}

	// Default chart type (value = portfolio value, performance = performance %)
	if widget.ChartType == "" {
		widget.ChartType = "value"
	}

	// Default chart height
	if widget.ChartHeight == 0 {
		widget.ChartHeight = 100
	}

	return nil
}

func (widget *ghostfolioWidget) update(ctx context.Context) {
	portfolioData, err := fetchAllGhostfolioPortfolios(
		widget.URL,
		widget.AccessToken,
		widget.AllowInsecure,
	)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.PortfolioData = portfolioData
}

func (widget *ghostfolioWidget) Render() template.HTML {
	return widget.renderTemplate(widget, ghostfolioWidgetTemplate)
}

// Authentication response from Ghostfolio
type ghostfolioAuthResponse struct {
	AuthToken string `json:"authToken"`
}

// Performance API response
type ghostfolioPerformanceResponse struct {
	Chart []ghostfolioChartPoint `json:"chart"`
}

type ghostfolioChartPoint struct {
	Date                    string  `json:"date"`
	Value                   float64 `json:"value"`
	NetPerformance          float64 `json:"netPerformance"`
	NetPerformanceInPercent float64 `json:"netPerformanceInPercentage"`
}

// Account API response to get currency
type ghostfolioAccountResponse struct {
	Accounts []struct {
		Currency string `json:"currency"`
	} `json:"accounts"`
}

// fetchAllGhostfolioPortfolios fetches portfolio data for all range options
func fetchAllGhostfolioPortfolios(baseURL, accessToken string, allowInsecure bool) (map[string]*ghostfolioPortfolio, error) {
	client := ternary(allowInsecure, defaultInsecureHTTPClient, defaultHTTPClient)

	// Step 1: Authenticate to get bearer token
	bearerToken, err := ghostfolioAuthenticate(client, baseURL, accessToken)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Step 2: Fetch account info to get currency
	accountURL := fmt.Sprintf("%s/api/v1/account", baseURL)
	accountReq, err := http.NewRequest("GET", accountURL, nil)
	if err != nil {
		return nil, err
	}
	accountReq.Header.Set("Authorization", "Bearer "+bearerToken)
	accountReq.Header.Set("Content-Type", "application/json")

	accountResp, err := decodeJsonFromRequest[ghostfolioAccountResponse](client, accountReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}

	// Extract currency from accounts
	currency := "EUR"
	if len(accountResp.Accounts) > 0 && accountResp.Accounts[0].Currency != "" {
		currency = accountResp.Accounts[0].Currency
	}

	// Convert currency code to symbol
	currencySymbol, exists := currencyToSymbol[strings.ToUpper(currency)]
	if !exists {
		currencySymbol = currency
	}

	// Step 3: Fetch performance data for all ranges
	portfolioData := make(map[string]*ghostfolioPortfolio)

	for _, rangeOpt := range ghostfolioRangeOptions {
		portfolio, err := fetchGhostfolioPerformance(client, baseURL, bearerToken, rangeOpt.Value, currencySymbol)
		if err != nil {
			// Continue with other ranges even if one fails
			portfolioData[rangeOpt.Value] = &ghostfolioPortfolio{HasData: false}
			continue
		}
		portfolioData[rangeOpt.Value] = portfolio
	}

	// Check if at least one range has data
	hasAnyData := false
	for _, p := range portfolioData {
		if p.HasData {
			hasAnyData = true
			break
		}
	}

	if !hasAnyData {
		return nil, fmt.Errorf("no portfolio data available for any range")
	}

	return portfolioData, nil
}

func fetchGhostfolioPerformance(client requestDoer, baseURL, bearerToken, rangeParam, currencySymbol string) (*ghostfolioPortfolio, error) {
	performanceURL := fmt.Sprintf("%s/api/v2/portfolio/performance?range=%s", baseURL, rangeParam)
	performanceReq, err := http.NewRequest("GET", performanceURL, nil)
	if err != nil {
		return nil, err
	}
	performanceReq.Header.Set("Authorization", "Bearer "+bearerToken)
	performanceReq.Header.Set("Content-Type", "application/json")

	performanceResp, err := decodeJsonFromRequest[ghostfolioPerformanceResponse](client, performanceReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch performance: %w", err)
	}

	// Process chart data
	if len(performanceResp.Chart) == 0 {
		return &ghostfolioPortfolio{HasData: false}, nil
	}

	// Extract values for the SVG charts and build chart data for tooltips
	values := make([]float64, len(performanceResp.Chart))
	performanceValues := make([]float64, len(performanceResp.Chart))
	chartData := make([]ghostfolioChartDataPoint, len(performanceResp.Chart))

	for i, point := range performanceResp.Chart {
		values[i] = point.Value
		performanceValues[i] = point.NetPerformanceInPercent * 100 // Convert to percentage
		chartData[i] = ghostfolioChartDataPoint{
			Date:           point.Date,
			Value:          point.Value,
			PerformancePct: point.NetPerformanceInPercent * 100,
		}
	}

	// Get latest data point for summary
	latest := performanceResp.Chart[len(performanceResp.Chart)-1]

	// Generate SVG polyline coordinates for both charts
	chartPoints := svgPolylineCoordsFromYValues(100, 50, maybeCopySliceWithoutZeroValues(values))
	performanceChartPoints := svgPolylineCoordsFromYValues(100, 50, maybeCopySliceWithoutZeroValues(performanceValues))

	return &ghostfolioPortfolio{
		TotalValue:             latest.Value,
		NetPerformance:         latest.NetPerformance,
		NetPerformancePct:      latest.NetPerformanceInPercent * 100, // Convert to percentage
		Currency:               currencySymbol,
		ChartPoints:            chartPoints,
		PerformanceChartPoints: performanceChartPoints,
		ChartData:              chartData,
		HasData:                true,
	}, nil
}

func ghostfolioAuthenticate(client requestDoer, baseURL, accessToken string) (string, error) {
	authURL := fmt.Sprintf("%s/api/v1/auth/anonymous", baseURL)

	body, err := json.Marshal(map[string]string{
		"accessToken": accessToken,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", authURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authentication returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var authResp ghostfolioAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.AuthToken == "" {
		return "", fmt.Errorf("empty auth token received")
	}

	return authResp.AuthToken, nil
}
