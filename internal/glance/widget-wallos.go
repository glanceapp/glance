package glance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	wallosWidgetType        = "wallos"
	wallosSubscriptionsPath = "/api/subscriptions/get_subscriptions.php"
	wallosMonthlyCostPath   = "/api/subscriptions/get_monthly_cost.php"
	wallosDefaultDuration   = 30 * time.Minute // Default widget refresh frequency
	wallosDateFormat        = "2006-01-02"     // Expected date format from Wallos API
	// Timezone used for relative date calculations (e.g., "today", "tomorrow").
	// Consider making this configurable globally in Glance if possible.
	wallosTimeZone = "America/Toronto"
)

var wallosWidgetTemplate = mustParseTemplate("wallos.html", "widget-base.html")

// wallosWidget represents the state and configuration for the Wallos widget.
type wallosWidget struct {
	widgetBase `yaml:",inline"` // Embed base functionality

	// Wallos specific configuration
	InstanceURL string `yaml:"url"`
	Token       string `yaml:"key"`

	// Visibility Configuration
	HideActiveColumn  bool `yaml:"hide_active,omitempty"`
	HideMonthlyColumn bool `yaml:"hide_monthly,omitempty"`
	HideYearlyColumn  bool `yaml:"hide_yearly,omitempty"`
	HideUpcoming      bool `yaml:"hide_upcoming,omitempty"`
	ShowUpcomingPrice bool `yaml:"show_upcoming_price,omitempty"`

	// Data fetched and processed for the template
	Data WallosData `yaml:"-"`
}

// WallosData holds the specific fields needed by the wallos.html template.
type WallosData struct {
	ActiveCount            int       `json:"-"`
	MonthlyCost            string    `json:"-"` // Formatted AVERAGE monthly cost ($symbol + number)
	YearlyCost             string    `json:"-"` // Formatted TOTAL yearly cost ($symbol + number)
	CurrencyCode           string    `json:"-"` // e.g., "CAD"
	CurrencySymbol         string    `json:"-"` // e.g., "$"
	UpcomingName           string    `json:"-"`
	UpcomingDate           string    `json:"-"` // Original date string
	UpcomingDateTime       time.Time `json:"-"` // Parsed date (UTC noon) for calculations
	UpcomingRelativeTime   string    `json:"-"` // e.g., "today", "in 5 days"
	UpcomingPriceFormatted string    `json:"-"` // Formatted price (e.g., "$6.99"), empty if disabled
	BaseURL                string    `json:"-"`
}

// initialize sets default values and validates the configuration.
func (widget *wallosWidget) initialize() error {
	widget.withTitle("Wallos")
	if widget.InstanceURL == "" {
		return fmt.Errorf("%s: url is required", wallosWidgetType)
	}
	if widget.Token == "" {
		return fmt.Errorf("%s: key (api_key) is required", wallosWidgetType)
	}
	_, err := url.ParseRequestURI(widget.InstanceURL)
	if err != nil {
		return fmt.Errorf("%s: invalid url format: %w", wallosWidgetType, err)
	}
	widget.InstanceURL = strings.TrimSuffix(widget.InstanceURL, "/")
	widget.withCacheDuration(wallosDefaultDuration)
	widget.Data.BaseURL = widget.InstanceURL
	widget.ContentAvailable = false
	return nil
}

// update fetches subscription data and calculates yearly/average costs by calling the monthly API 12 times.
func (widget *wallosWidget) update(ctx context.Context) {
	var wg sync.WaitGroup
	var subsResp *WallosSubscriptionResponse
	var subsErr error
	var totalYearlyCost float64
	var currencySymbol string
	var costErr error

	// Fetch subscriptions and aggregated costs concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		slog.Debug("Wallos: starting fetch subscriptions", "widget_id", widget.ID)
		subsResp, subsErr = fetchWallosSubscriptions(ctx, widget.InstanceURL, widget.Token)
		if subsErr != nil {
			slog.Error("Wallos: error during fetch subscriptions", "widget_id", widget.ID, "error", subsErr)
		} else {
			slog.Debug("Wallos: finished fetch subscriptions", "widget_id", widget.ID)
		}
	}()
	go func() {
		defer wg.Done()
		slog.Debug("Wallos: starting fetch yearly costs (12 calls)", "widget_id", widget.ID)
		totalYearlyCost, currencySymbol, costErr = fetchAndCalculateYearlyCosts(ctx, widget.InstanceURL, widget.Token)
		if costErr != nil {
			slog.Error("Wallos: error during fetch yearly costs", "widget_id", widget.ID, "error", costErr)
		} else {
			slog.Debug("Wallos: finished fetch yearly costs", "widget_id", widget.ID)
		}
	}()
	wg.Wait()

	// Process results
	widget.Error = nil
	widget.Notice = nil
	hasSubscriptionData := false
	hasCostData := false
	// Clear previous data
	widget.Data = WallosData{BaseURL: widget.Data.BaseURL} // Reset data struct, keep BaseURL
	widget.Data.UpcomingName = "N/A"                       // Default
	widget.Data.MonthlyCost = "N/A"
	widget.Data.YearlyCost = "N/A"

	// Process aggregated cost results (Average Monthly, Total Yearly)
	if widget.canContinueUpdateAfterHandlingErr(costErr) && currencySymbol != "" {
		widget.Data.CurrencySymbol = currencySymbol
		avgMonthlyCost := totalYearlyCost / 12.0

		// Format costs using intl printer (or fallback)
		var currentPrinter *message.Printer = message.NewPrinter(language.English)
		if intl != nil {
			currentPrinter = intl
		} else {
			slog.Warn("Wallos: Global 'intl' message printer not available, using default English formatting", "widget_id", widget.ID)
		}
		formattedAvgMonthly := currentPrinter.Sprintf("%.2f", avgMonthlyCost)
		widget.Data.MonthlyCost = widget.Data.CurrencySymbol + formattedAvgMonthly // AVERAGE
		formattedTotalYearly := currentPrinter.Sprintf("%.2f", totalYearlyCost)
		widget.Data.YearlyCost = widget.Data.CurrencySymbol + formattedTotalYearly // TOTAL SUM
		hasCostData = true
	}

	// Process subscription results (Active Count, Upcoming Event)
	if widget.canContinueUpdateAfterHandlingErr(subsErr) && subsResp != nil {
		if !subsResp.Success {
			notesMsg := "unknown API error"
			if len(subsResp.Notes) > 0 {
				notesMsg = strings.Join(subsResp.Notes, "; ")
			}
			apiErr := fmt.Errorf("subscriptions API error: %s", notesMsg)
			widget.withNotice(apiErr)
			slog.Warn("Wallos: subscription API reported failure", "widget_id", widget.ID, "notes", subsResp.Notes)
		} else {
			widget.Data.ActiveCount = len(subsResp.Subscriptions)
			if !widget.HideUpcoming && len(subsResp.Subscriptions) > 0 {
				firstSub := subsResp.Subscriptions[0]
				widget.Data.UpcomingName = firstSub.Name
				widget.Data.UpcomingDate = firstSub.NextPayment
				parsedTime, err := time.Parse(wallosDateFormat, firstSub.NextPayment)
				if err != nil {
					slog.Warn("Wallos: could not parse upcoming payment date", "widget_id", widget.ID, "date_string", firstSub.NextPayment, "error", err)
					widget.Data.UpcomingDateTime = time.Time{}
					widget.Data.UpcomingRelativeTime = ""
				} else {
					targetTimeUTC := time.Date(parsedTime.Year(), parsedTime.Month(), parsedTime.Day(), 12, 0, 0, 0, time.UTC)
					widget.Data.UpcomingDateTime = targetTimeUTC
					widget.Data.UpcomingRelativeTime = formatRelativeDate(targetTimeUTC)
					// Format Upcoming Price if requested and symbol is available
					if widget.ShowUpcomingPrice && widget.Data.CurrencySymbol != "" {
						var currentPrinter *message.Printer = message.NewPrinter(language.English)
						if intl != nil {
							currentPrinter = intl
						}
						formattedPrice := currentPrinter.Sprintf("%.2f", firstSub.Price)
						widget.Data.UpcomingPriceFormatted = widget.Data.CurrencySymbol + formattedPrice
					} else if widget.ShowUpcomingPrice && widget.Data.CurrencySymbol == "" {
						slog.Warn("Wallos: ShowUpcomingPrice is true, but CurrencySymbol is missing; cannot format price.", "widget_id", widget.ID)
					}
				}
			} else if widget.HideUpcoming {
				widget.Data.UpcomingName = "N/A"
				widget.Data.UpcomingRelativeTime = ""
				widget.Data.UpcomingPriceFormatted = ""
			}
			hasSubscriptionData = true
		}
	}

	// Final status check
	if widget.Error == nil && (hasSubscriptionData || hasCostData) {
		widget.ContentAvailable = true
		slog.Debug("Wallos: Update successful, content available", "widget_id", widget.ID, "notice", widget.Notice)
	} else {
		widget.ContentAvailable = false
		if widget.Error == nil {
			if widget.Notice != nil {
				widget.Error = widget.Notice
				slog.Warn("Wallos: Update resulted in no content, promoting notice to error", "widget_id", widget.ID, "error", widget.Error)
			} else {
				widget.Error = errors.New("failed to fetch any data from Wallos API")
				slog.Error("Wallos: Update failed with no data and no specific error/notice", "widget_id", widget.ID)
			}
		} else {
			slog.Error("Wallos: Update failed with blocking error", "widget_id", widget.ID, "error", widget.Error)
		}
	}
	slog.Debug("Wallos: update cycle complete", "widget_id", widget.ID, "next_update", widget.nextUpdate.Format(time.RFC3339), "error", widget.Error, "notice", widget.Notice, "content_available", widget.ContentAvailable)
}

// Render generates the HTML representation of the widget.
func (widget *wallosWidget) Render() template.HTML {
	return widget.renderTemplate(widget, wallosWidgetTemplate)
}

// --- API Data Structures ---

// WallosSubscription represents a single subscription entry from the API.
type WallosSubscription struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	Logo              string  `json:"logo,omitempty"`
	Price             float64 `json:"price"` // Price (converted if convert_currency=true)
	CurrencyID        int     `json:"currency_id"`
	NextPayment       string  `json:"next_payment"` // Format: "YYYY-MM-DD"
	Cycle             int     `json:"cycle"`        // 1:Daily, 2:Weekly, 3:Monthly, 4:Yearly
	Frequency         int     `json:"frequency"`
	Notes             string  `json:"notes"`
	PaymentMethodID   int     `json:"payment_method_id"`
	PayerUserID       int     `json:"payer_user_id"`
	CategoryID        int     `json:"category_id"`
	Notify            int     `json:"notify"`
	URL               string  `json:"url"`
	Inactive          int     `json:"inactive"`
	NotifyDaysBefore  *int    `json:"notify_days_before,omitempty"` // Pointer to handle null
	UserID            int     `json:"user_id"`
	CancellationDate  string  `json:"cancellation_date"`
	ReplacementSubID  *int    `json:"replacement_subscription_id,omitempty"` // Pointer for null
	StartDate         string  `json:"start_date"`
	AutoRenew         int     `json:"auto_renew"`
	CategoryName      string  `json:"category_name"`
	PayerUserName     string  `json:"payer_user_name"`
	PaymentMethodName string  `json:"payment_method_name"`
}

// WallosSubscriptionResponse is the structure of the /get_subscriptions.php response.
type WallosSubscriptionResponse struct {
	Success       bool                 `json:"success"`
	Title         string               `json:"title,omitempty"`
	Subscriptions []WallosSubscription `json:"subscriptions"`
	Notes         []string             `json:"notes"` // Root level notes array
}

// WallosMonthlyCostResponse is the structure of the /get_monthly_cost.php response.
type WallosMonthlyCostResponse struct {
	Success              bool     `json:"success"`
	Title                string   `json:"title"`
	MonthlyCost          string   `json:"monthly_cost"` // Raw cost string
	LocalizedMonthlyCost string   `json:"localized_monthly_cost"`
	CurrencyCode         string   `json:"currency_code"`
	CurrencySymbol       string   `json:"currency_symbol"`
	Notes                []string `json:"notes"` // Root level notes array
}

// --- API Helper Functions ---

// buildWallosAPIRequest constructs an authenticated HTTP request.
func buildWallosAPIRequest(ctx context.Context, instanceURL, token, method, path string, queryParams url.Values) (*http.Request, error) {
	fullURL, err := url.Parse(instanceURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base url '%s': %w", instanceURL, err)
	}
	relPath, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parsing relative path '%s': %w", path, err)
	}
	fullURL = fullURL.ResolveReference(relPath)
	if queryParams == nil {
		queryParams = url.Values{}
	}
	queryParams.Set("api_key", token)
	fullURL.RawQuery = queryParams.Encode()
	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", fullURL.Redacted(), err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GlanceApp-Widget/1.0 (Wallos)")
	return req, nil
}

// executeAndDecode performs the HTTP request and decodes the JSON response.
func executeAndDecode[T any](ctx context.Context, req *http.Request) (*T, error) {
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("request %w: %s", ctx.Err(), req.URL.Redacted())
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, fmt.Errorf("http client url error for %s: %w", req.URL.Redacted(), err)
		}
		return nil, fmt.Errorf("http request failed for %s: %w", req.URL.Redacted(), err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body for %s: %w", req.URL.Redacted(), err)
	}
	bodyStr := string(bodyBytes)
	if resp.StatusCode != http.StatusOK {
		bodyPreview := bodyStr
		if len(bodyPreview) > 300 {
			bodyPreview = bodyPreview[:300] + "..."
		}
		if strings.HasPrefix(strings.TrimSpace(bodyPreview), "<") && !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
			return nil, fmt.Errorf("api request to %s returned status %d with non-JSON body: %s", req.URL.Redacted(), resp.StatusCode, bodyPreview)
		}
		return nil, fmt.Errorf("api request to %s returned status %d: %s", req.URL.Redacted(), resp.StatusCode, bodyPreview)
	}
	trimmedBody := strings.TrimSpace(bodyStr)
	if strings.Contains(bodyStr, "<b>Warning</b>:") && !(strings.HasPrefix(trimmedBody, "{") || strings.HasPrefix(trimmedBody, "[")) {
		bodyPreview := bodyStr
		if len(bodyPreview) > 300 {
			bodyPreview = bodyPreview[:300] + "..."
		}
		return nil, fmt.Errorf("api response from %s appears to contain PHP warnings and is not valid JSON: %s", req.URL.Redacted(), bodyPreview)
	}
	var result T
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		bodyPreview := bodyStr
		if len(bodyPreview) > 300 {
			bodyPreview = bodyPreview[:300] + "..."
		}
		return nil, fmt.Errorf("decoding json response from %s: %w - Body: %s", req.URL.Redacted(), err, bodyPreview)
	}
	return &result, nil
}

// fetchWallosSubscriptions retrieves active, sorted subscriptions (currency converted).
func fetchWallosSubscriptions(ctx context.Context, instanceURL, token string) (*WallosSubscriptionResponse, error) {
	params := url.Values{}
	params.Set("state", "0")
	params.Set("sort", "next_payment")
	params.Set("convert_currency", "true")
	req, err := buildWallosAPIRequest(ctx, instanceURL, token, http.MethodGet, wallosSubscriptionsPath, params)
	if err != nil {
		return nil, err
	}
	return executeAndDecode[WallosSubscriptionResponse](ctx, req)
}

// fetchWallosMonthlyCost retrieves the calculated monthly cost for a specific month/year.
func fetchWallosMonthlyCost(ctx context.Context, instanceURL, token string, month time.Month, year int) (*WallosMonthlyCostResponse, error) {
	params := url.Values{}
	params.Set("month", strconv.Itoa(int(month)))
	params.Set("year", strconv.Itoa(year))
	req, err := buildWallosAPIRequest(ctx, instanceURL, token, http.MethodGet, wallosMonthlyCostPath, params)
	if err != nil {
		return nil, err
	}
	return executeAndDecode[WallosMonthlyCostResponse](ctx, req)
}

// fetchAndCalculateYearlyCosts calls the monthly cost endpoint 12 times concurrently
// and returns the total sum and the currency symbol. Returns an aggregate error if any call fails.
// WARNING: This makes 12 API calls per update cycle.
func fetchAndCalculateYearlyCosts(ctx context.Context, instanceURL, token string) (totalCost float64, currencySymbol string, err error) {
	type monthResult struct {
		cost   float64
		symbol string
		err    error
	}
	results := make([]monthResult, 12)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var collectedErrors []error
	firstSuccess := true
	now := time.Now()
	// Reasonable timeout for 12 concurrent calls
	opCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	wg.Add(12)
	for i := 0; i < 12; i++ {
		go func(monthOffset int) {
			defer wg.Done()
			targetDate := now.AddDate(0, monthOffset, 0)
			targetMonth := targetDate.Month()
			targetYear := targetDate.Year()
			costResp, fetchErr := fetchWallosMonthlyCost(opCtx, instanceURL, token, targetMonth, targetYear)
			currentResult := monthResult{cost: 0.0, symbol: ""} // Initialize cost to 0 for safety
			if fetchErr != nil {
				currentResult.err = fmt.Errorf("failed fetching cost for %s %d: %w", targetMonth.String(), targetYear, fetchErr)
			} else if !costResp.Success {
				currentResult.err = fmt.Errorf("api error for %s %d: %v", targetMonth.String(), targetYear, costResp.Notes)
			} else {
				monthCostFloat, parseErr := strconv.ParseFloat(costResp.MonthlyCost, 64)
				if parseErr != nil {
					currentResult.err = fmt.Errorf("failed parsing cost ('%s') for %s %d: %w", costResp.MonthlyCost, targetMonth.String(), targetYear, parseErr)
				} else {
					currentResult.cost = monthCostFloat
					currentResult.symbol = costResp.CurrencySymbol
				}
			}
			// Safely write result - Although slice access by index is safe here after WaitGroup, mutex prevents race condition warnings
			mu.Lock()
			results[monthOffset] = currentResult
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Check overall context after waiting
	if opCtx.Err() != nil {
		return 0.0, "", fmt.Errorf("failed fetching costs for 12 months: %w", opCtx.Err())
	}

	// Aggregate results and check for errors
	totalCost = 0.0
	currencySymbol = ""
	for i, res := range results {
		if res.err != nil {
			collectedErrors = append(collectedErrors, res.err)
			slog.Error("Wallos: error in concurrent cost fetch", "month_offset", i, "error", res.err)
			continue
		}
		totalCost += res.cost
		if firstSuccess {
			currencySymbol = res.symbol
			firstSuccess = false
		} else if currencySymbol != res.symbol {
			slog.Warn("Wallos: Currency symbol mismatch across monthly costs", "expected", currencySymbol, "got", res.symbol, "month_offset", i)
		}
	}

	if len(collectedErrors) > 0 {
		// Fail aggregation if any month failed
		err = fmt.Errorf("encountered %d errors fetching monthly costs: %v", len(collectedErrors), collectedErrors)
		return 0.0, "", err
	}
	if firstSuccess { // Means no calls succeeded
		return 0.0, "", errors.New("all 12 monthly cost API calls failed or returned no symbol")
	}

	slog.Debug("Wallos: finished processing 12 months concurrent cost fetches", "total_yearly", totalCost, "symbol", currencySymbol)
	return totalCost, currencySymbol, nil
}

// --- Relative Date Formatting Helper ---

// formatRelativeDate formats the target time relative to today using the configured timezone.
func formatRelativeDate(targetTimeUTC time.Time) string {
	if targetTimeUTC.IsZero() {
		return ""
	}
	loc, err := time.LoadLocation(wallosTimeZone)
	if err != nil {
		loc = time.Local
		slog.Warn("Wallos: could not load timezone, using system local", "timezone", wallosTimeZone, "error", err)
	}
	now := time.Now().In(loc)
	year, month, day := now.Date()
	todayStart := time.Date(year, month, day, 0, 0, 0, 0, loc)
	targetTimeInLoc := targetTimeUTC.In(loc)
	targetYear, targetMonth, targetDay := targetTimeInLoc.Date()
	targetStart := time.Date(targetYear, targetMonth, targetDay, 0, 0, 0, 0, loc)
	hoursDiff := targetStart.Sub(todayStart).Hours()
	daysDiff := int(math.Round(hoursDiff / 24.0))
	switch {
	case daysDiff == 0:
		return "today"
	case daysDiff == 1:
		return "tomorrow"
	case daysDiff > 1:
		return fmt.Sprintf("in %d days", daysDiff)
	case daysDiff == -1:
		return "yesterday"
	case daysDiff < -1:
		return fmt.Sprintf("%d days ago", -daysDiff)
	default:
		return fmt.Sprintf("on %s", targetTimeInLoc.Format(wallosDateFormat))
	}
}
