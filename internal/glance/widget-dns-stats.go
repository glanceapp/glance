package glance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var dnsStatsWidgetTemplate = mustParseTemplate("dns-stats.html", "widget-base.html")

const (
	dnsStatsBars            = 8
	dnsStatsHoursSpan       = 24
	dnsStatsHoursPerBar int = dnsStatsHoursSpan / dnsStatsBars
)

type dnsStatsWidget struct {
	widgetBase `yaml:",inline"`

	TimeLabels      [8]string `yaml:"-"`
	Stats           *dnsStats `yaml:"-"`
	piholeSessionID string    `yaml:"-"`

	HourFormat     string `yaml:"hour-format"`
	HideGraph      bool   `yaml:"hide-graph"`
	HideTopDomains bool   `yaml:"hide-top-domains"`
	Service        string `yaml:"service"`
	AllowInsecure  bool   `yaml:"allow-insecure"`
	URL            string `yaml:"url"`
	Token          string `yaml:"token"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
}

const (
	dnsServiceAdguard    = "adguard"
	dnsServicePihole     = "pihole"
	dnsServiceTechnitium = "technitium"
	dnsServicePiholeV6   = "pihole-v6"
)

func makeDNSWidgetTimeLabels(format string) [8]string {
	now := time.Now()
	var labels [dnsStatsBars]string

	for h := dnsStatsHoursSpan; h > 0; h -= dnsStatsHoursPerBar {
		labels[7-(h/3-1)] = strings.ToLower(now.Add(-time.Duration(h) * time.Hour).Format(format))
	}

	return labels
}

func (widget *dnsStatsWidget) initialize() error {
	titleURL := strings.TrimRight(widget.URL, "/")
	switch widget.Service {
	case dnsServicePihole, dnsServicePiholeV6:
		titleURL = titleURL + "/admin"
	}

	widget.
		withTitle("DNS Stats").
		withTitleURL(titleURL).
		withCacheDuration(10 * time.Minute)

	switch widget.Service {
	case dnsServiceAdguard:
	case dnsServicePiholeV6:
	case dnsServicePihole:
	case dnsServiceTechnitium:
	default:
		return fmt.Errorf("service must be one of: %s, %s, %s, %s", dnsServiceAdguard, dnsServicePihole, dnsServicePiholeV6, dnsServiceTechnitium)
	}

	return nil
}

func (widget *dnsStatsWidget) update(ctx context.Context) {
	var stats *dnsStats
	var err error

	switch widget.Service {
	case dnsServiceAdguard:
		stats, err = fetchAdguardStats(widget.URL, widget.AllowInsecure, widget.Username, widget.Password, widget.HideGraph)
	case dnsServicePihole:
		stats, err = fetchPihole5Stats(widget.URL, widget.AllowInsecure, widget.Token, widget.HideGraph)
	case dnsServiceTechnitium:
		stats, err = fetchTechnitiumStats(widget.URL, widget.AllowInsecure, widget.Token, widget.HideGraph)
	case dnsServicePiholeV6:
		var newSessionID string
		stats, newSessionID, err = fetchPiholeStats(
			widget.URL,
			widget.AllowInsecure,
			widget.Password,
			widget.piholeSessionID,
			!widget.HideGraph,
			!widget.HideTopDomains,
		)
		if err == nil {
			widget.piholeSessionID = newSessionID
		}
	}

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.HourFormat == "24h" {
		widget.TimeLabels = makeDNSWidgetTimeLabels("15:00")
	} else {
		widget.TimeLabels = makeDNSWidgetTimeLabels("3PM")
	}

	widget.Stats = stats
}

func (widget *dnsStatsWidget) Render() template.HTML {
	return widget.renderTemplate(widget, dnsStatsWidgetTemplate)
}

type dnsStats struct {
	TotalQueries      int
	BlockedQueries    int // we don't actually use this anywhere in templates, maybe remove it later?
	BlockedPercent    int
	ResponseTime      int
	DomainsBlocked    int
	Series            [dnsStatsBars]dnsStatsSeries
	TopBlockedDomains []dnsStatsBlockedDomain
}

type dnsStatsSeries struct {
	Queries        int
	Blocked        int
	PercentTotal   int
	PercentBlocked int
}

type dnsStatsBlockedDomain struct {
	Domain         string
	PercentBlocked int
}

type adguardStatsResponse struct {
	TotalQueries      int              `json:"num_dns_queries"`
	QueriesSeries     []int            `json:"dns_queries"`
	BlockedQueries    int              `json:"num_blocked_filtering"`
	BlockedSeries     []int            `json:"blocked_filtering"`
	ResponseTime      float64          `json:"avg_processing_time"`
	TopBlockedDomains []map[string]int `json:"top_blocked_domains"`
}

func fetchAdguardStats(instanceURL string, allowInsecure bool, username, password string, noGraph bool) (*dnsStats, error) {
	requestURL := strings.TrimRight(instanceURL, "/") + "/control/stats"

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(username, password)

	var client = ternary(allowInsecure, defaultInsecureHTTPClient, defaultHTTPClient)
	responseJson, err := decodeJsonFromRequest[adguardStatsResponse](client, request)
	if err != nil {
		return nil, err
	}

	var topBlockedDomainsCount = min(len(responseJson.TopBlockedDomains), 5)

	stats := &dnsStats{
		TotalQueries:      responseJson.TotalQueries,
		BlockedQueries:    responseJson.BlockedQueries,
		ResponseTime:      int(responseJson.ResponseTime * 1000),
		TopBlockedDomains: make([]dnsStatsBlockedDomain, 0, topBlockedDomainsCount),
	}

	if stats.TotalQueries <= 0 {
		return stats, nil
	}

	stats.BlockedPercent = int(float64(responseJson.BlockedQueries) / float64(responseJson.TotalQueries) * 100)

	for i := range topBlockedDomainsCount {
		domain := responseJson.TopBlockedDomains[i]
		var firstDomain string

		for k := range domain {
			firstDomain = k
			break
		}

		if firstDomain == "" {
			continue
		}

		stats.TopBlockedDomains = append(stats.TopBlockedDomains, dnsStatsBlockedDomain{
			Domain: firstDomain,
		})

		if stats.BlockedQueries > 0 {
			stats.TopBlockedDomains[i].PercentBlocked = int(float64(domain[firstDomain]) / float64(responseJson.BlockedQueries) * 100)
		}
	}

	if noGraph {
		return stats, nil
	}

	queriesSeries := responseJson.QueriesSeries
	blockedSeries := responseJson.BlockedSeries

	if len(queriesSeries) > dnsStatsHoursSpan {
		queriesSeries = queriesSeries[len(queriesSeries)-dnsStatsHoursSpan:]
	} else if len(queriesSeries) < dnsStatsHoursSpan {
		queriesSeries = append(make([]int, dnsStatsHoursSpan-len(queriesSeries)), queriesSeries...)
	}

	if len(blockedSeries) > dnsStatsHoursSpan {
		blockedSeries = blockedSeries[len(blockedSeries)-dnsStatsHoursSpan:]
	} else if len(blockedSeries) < dnsStatsHoursSpan {
		blockedSeries = append(make([]int, dnsStatsHoursSpan-len(blockedSeries)), blockedSeries...)
	}

	maxQueriesInSeries := 0

	for i := range dnsStatsBars {
		queries := 0
		blocked := 0

		for j := range dnsStatsHoursPerBar {
			queries += queriesSeries[i*dnsStatsHoursPerBar+j]
			blocked += blockedSeries[i*dnsStatsHoursPerBar+j]
		}

		stats.Series[i] = dnsStatsSeries{
			Queries: queries,
			Blocked: blocked,
		}

		if queries > 0 {
			stats.Series[i].PercentBlocked = int(float64(blocked) / float64(queries) * 100)
		}

		if queries > maxQueriesInSeries {
			maxQueriesInSeries = queries
		}
	}

	for i := range dnsStatsBars {
		stats.Series[i].PercentTotal = int(float64(stats.Series[i].Queries) / float64(maxQueriesInSeries) * 100)
	}

	return stats, nil
}

// Legacy Pi-hole stats response (before v6)
type pihole5StatsResponse struct {
	TotalQueries      int                      `json:"dns_queries_today"`
	QueriesSeries     pihole5QueriesSeries     `json:"domains_over_time"`
	BlockedQueries    int                      `json:"ads_blocked_today"`
	BlockedSeries     map[int64]int            `json:"ads_over_time"`
	BlockedPercentage float64                  `json:"ads_percentage_today"`
	TopBlockedDomains pihole5TopBlockedDomains `json:"top_ads"`
	DomainsBlocked    int                      `json:"domains_being_blocked"`
}

// If the user has query logging disabled it's possible for domains_over_time to be returned as an
// empty array rather than a map which will prevent unmashalling the rest of the data so we use
// custom unmarshal behavior to fallback to an empty map.
// See https://github.com/glanceapp/glance/issues/289
type pihole5QueriesSeries map[int64]int

func (p *pihole5QueriesSeries) UnmarshalJSON(data []byte) error {
	temp := make(map[int64]int)

	err := json.Unmarshal(data, &temp)
	if err != nil {
		*p = make(pihole5QueriesSeries)
	} else {
		*p = temp
	}

	return nil
}

// If user has some level of privacy enabled on Pihole, `json:"top_ads"` is an empty array
// Use custom unmarshal behavior to avoid not getting the rest of the valid data when unmarshalling
type pihole5TopBlockedDomains map[string]int

func (p *pihole5TopBlockedDomains) UnmarshalJSON(data []byte) error {
	// NOTE: do not change to piholeTopBlockedDomains type here or it will cause a stack overflow
	// because of the UnmarshalJSON method getting called recursively
	temp := make(map[string]int)

	err := json.Unmarshal(data, &temp)
	if err != nil {
		*p = make(pihole5TopBlockedDomains)
	} else {
		*p = temp
	}

	return nil
}

func fetchPihole5Stats(instanceURL string, allowInsecure bool, token string, noGraph bool) (*dnsStats, error) {
	if token == "" {
		return nil, errors.New("missing API token")
	}

	requestURL := strings.TrimRight(instanceURL, "/") +
		"/admin/api.php?summaryRaw&topItems&overTimeData10mins&auth=" + token

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	var client = ternary(allowInsecure, defaultInsecureHTTPClient, defaultHTTPClient)
	responseJson, err := decodeJsonFromRequest[pihole5StatsResponse](client, request)
	if err != nil {
		return nil, err
	}

	stats := &dnsStats{
		TotalQueries:   responseJson.TotalQueries,
		BlockedQueries: responseJson.BlockedQueries,
		BlockedPercent: int(responseJson.BlockedPercentage),
		DomainsBlocked: responseJson.DomainsBlocked,
	}

	if len(responseJson.TopBlockedDomains) > 0 {
		domains := make([]dnsStatsBlockedDomain, 0, len(responseJson.TopBlockedDomains))

		for domain, count := range responseJson.TopBlockedDomains {
			domains = append(domains, dnsStatsBlockedDomain{
				Domain:         domain,
				PercentBlocked: int(float64(count) / float64(responseJson.BlockedQueries) * 100),
			})
		}

		sort.Slice(domains, func(a, b int) bool {
			return domains[a].PercentBlocked > domains[b].PercentBlocked
		})

		stats.TopBlockedDomains = domains[:min(len(domains), 5)]
	}

	if noGraph {
		return stats, nil
	}

	// Pihole _should_ return data for the last 24 hours in a 10 minute interval, 6*24 = 144
	if len(responseJson.QueriesSeries) != 144 || len(responseJson.BlockedSeries) != 144 {
		slog.Warn(
			"DNS stats for pihole: did not get expected 144 data points",
			"len(queries)", len(responseJson.QueriesSeries),
			"len(blocked)", len(responseJson.BlockedSeries),
		)
		return stats, nil
	}

	var lowestTimestamp int64 = 0
	for timestamp := range responseJson.QueriesSeries {
		if lowestTimestamp == 0 || timestamp < lowestTimestamp {
			lowestTimestamp = timestamp
		}
	}

	maxQueriesInSeries := 0

	for i := range dnsStatsBars {
		queries := 0
		blocked := 0

		for j := range 18 {
			index := lowestTimestamp + int64(i*10800+j*600)

			queries += responseJson.QueriesSeries[index]
			blocked += responseJson.BlockedSeries[index]
		}

		if queries > maxQueriesInSeries {
			maxQueriesInSeries = queries
		}

		stats.Series[i] = dnsStatsSeries{
			Queries: queries,
			Blocked: blocked,
		}

		if queries > 0 {
			stats.Series[i].PercentBlocked = int(float64(blocked) / float64(queries) * 100)
		}
	}

	for i := range dnsStatsBars {
		stats.Series[i].PercentTotal = int(float64(stats.Series[i].Queries) / float64(maxQueriesInSeries) * 100)
	}

	return stats, nil
}

func fetchPiholeStats(
	instanceURL string,
	allowInsecure bool,
	password string,
	sessionID string,
	includeGraph bool,
	includeTopDomains bool,
) (*dnsStats, string, error) {
	instanceURL = strings.TrimRight(instanceURL, "/")
	var client = ternary(allowInsecure, defaultInsecureHTTPClient, defaultHTTPClient)

	fetchNewSessionID := func() error {
		newSessionID, err := fetchPiholeSessionID(instanceURL, client, password)
		if err != nil {
			return err
		}
		sessionID = newSessionID
		return nil
	}

	if sessionID == "" {
		if err := fetchNewSessionID(); err != nil {
			slog.Error("Failed to fetch Pihole v6 session ID", "error", err)
			return nil, "", fmt.Errorf("fetching session ID: %v", err)
		}
	} else {
		isValid, err := checkPiholeSessionIDIsValid(instanceURL, client, sessionID)
		if err != nil {
			slog.Error("Failed to check Pihole v6 session ID validity", "error", err)
			return nil, "", fmt.Errorf("checking session ID: %v", err)
		}

		if !isValid {
			if err := fetchNewSessionID(); err != nil {
				slog.Error("Failed to renew Pihole v6 session ID", "error", err)
				return nil, "", fmt.Errorf("renewing session ID: %v", err)
			}
		}
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type statsResponseJson struct {
		Queries struct {
			Total          int     `json:"total"`
			Blocked        int     `json:"blocked"`
			PercentBlocked float64 `json:"percent_blocked"`
		} `json:"queries"`
		Gravity struct {
			DomainsBlocked int `json:"domains_being_blocked"`
		} `json:"gravity"`
	}

	statsRequest, _ := http.NewRequestWithContext(ctx, "GET", instanceURL+"/api/stats/summary", nil)
	statsRequest.Header.Set("x-ftl-sid", sessionID)

	var statsResponse statsResponseJson
	var statsErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		statsResponse, statsErr = decodeJsonFromRequest[statsResponseJson](client, statsRequest)
		if statsErr != nil {
			cancel()
		}
	}()

	type seriesResponseJson struct {
		History []struct {
			Timestamp int64 `json:"timestamp"`
			Total     int   `json:"total"`
			Blocked   int   `json:"blocked"`
		} `json:"history"`
	}

	var seriesResponse seriesResponseJson
	var seriesErr error

	if includeGraph {
		seriesRequest, _ := http.NewRequestWithContext(ctx, "GET", instanceURL+"/api/history", nil)
		seriesRequest.Header.Set("x-ftl-sid", sessionID)

		wg.Add(1)
		go func() {
			defer wg.Done()
			seriesResponse, seriesErr = decodeJsonFromRequest[seriesResponseJson](client, seriesRequest)
		}()
	}

	type topDomainsResponseJson struct {
		Domains []struct {
			Domain string `json:"domain"`
			Count  int    `json:"count"`
		} `json:"domains"`
		TotalQueries   int     `json:"total_queries"`
		BlockedQueries int     `json:"blocked_queries"`
		Took           float64 `json:"took"`
	}

	var topDomainsResponse topDomainsResponseJson
	var topDomainsErr error

	if includeTopDomains {
		topDomainsRequest, _ := http.NewRequestWithContext(ctx, "GET", instanceURL+"/api/stats/top_domains?blocked=true", nil)
		topDomainsRequest.Header.Set("x-ftl-sid", sessionID)

		wg.Add(1)
		go func() {
			defer wg.Done()
			topDomainsResponse, topDomainsErr = decodeJsonFromRequest[topDomainsResponseJson](client, topDomainsRequest)
		}()
	}

	wg.Wait()
	partialContent := false

	if statsErr != nil {
		return nil, "", fmt.Errorf("fetching stats: %v", statsErr)
	}

	if includeGraph && seriesErr != nil {
		slog.Error("Failed to fetch Pihole v6 graph data", "error", seriesErr)
		partialContent = true
	}

	if includeTopDomains && topDomainsErr != nil {
		slog.Error("Failed to fetch Pihole v6 top domains", "error", topDomainsErr)
		partialContent = true
	}

	stats := &dnsStats{
		TotalQueries:   statsResponse.Queries.Total,
		BlockedQueries: statsResponse.Queries.Blocked,
		BlockedPercent: int(statsResponse.Queries.PercentBlocked),
		DomainsBlocked: statsResponse.Gravity.DomainsBlocked,
	}

	if includeGraph && seriesErr == nil {
		if len(seriesResponse.History) != 145 {
			slog.Error(
				"Pihole v6 graph data has unexpected length",
				"length", len(seriesResponse.History),
				"expected", 145,
			)
			partialContent = true
		} else {
			// The API from v5 used to return 144 data points, but v6 returns 145.
			// We only show data from the last 24 hours hours, Pihole returns data
			// points in a 10 minute interval, 24*(60/10) = 144. Why is there an extra
			// data point? I don't know, but we'll just ignore the first one since it's
			// the oldest data point.
			history := seriesResponse.History[1:]

			const interval = 10
			const dataPointsPerBar = dnsStatsHoursPerBar * (60 / interval)

			maxQueriesInSeries := 0

			for i := range dnsStatsBars {
				queries := 0
				blocked := 0
				for j := range dataPointsPerBar {
					index := i*dataPointsPerBar + j
					queries += history[index].Total
					blocked += history[index].Blocked
				}
				if queries > maxQueriesInSeries {
					maxQueriesInSeries = queries
				}
				stats.Series[i] = dnsStatsSeries{
					Queries: queries,
					Blocked: blocked,
				}
				if queries > 0 {
					stats.Series[i].PercentBlocked = int(float64(blocked) / float64(queries) * 100)
				}
			}

			for i := range dnsStatsBars {
				stats.Series[i].PercentTotal = int(float64(stats.Series[i].Queries) / float64(maxQueriesInSeries) * 100)
			}
		}
	}

	if includeTopDomains && topDomainsErr == nil && len(topDomainsResponse.Domains) > 0 {
		domains := make([]dnsStatsBlockedDomain, 0, len(topDomainsResponse.Domains))
		for i := range topDomainsResponse.Domains {
			d := &topDomainsResponse.Domains[i]
			domains = append(domains, dnsStatsBlockedDomain{
				Domain:         d.Domain,
				PercentBlocked: int(float64(d.Count) / float64(statsResponse.Queries.Blocked) * 100),
			})
		}

		sort.Slice(domains, func(a, b int) bool {
			return domains[a].PercentBlocked > domains[b].PercentBlocked
		})
		stats.TopBlockedDomains = domains[:min(len(domains), 5)]
	}

	return stats, sessionID, ternary(partialContent, errPartialContent, nil)
}

func fetchPiholeSessionID(instanceURL string, client *http.Client, password string) (string, error) {
	requestBody := []byte(`{"password":"` + password + `"}`)

	request, err := http.NewRequest("POST", instanceURL+"/api/auth", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("creating authentication request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("sending authentication request: %v", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("reading authentication response: %v", err)
	}

	var jsonResponse struct {
		Session struct {
			SID     string `json:"sid"`
			Message string `json:"message"`
		} `json:"session"`
	}

	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return "", fmt.Errorf("parsing authentication response: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"authentication request returned status %s with message '%s'",
			response.Status, jsonResponse.Session.Message,
		)
	}

	if jsonResponse.Session.SID == "" {
		return "", fmt.Errorf(
			"authentication response returned empty session ID, status code %d, message '%s'",
			response.StatusCode, jsonResponse.Session.Message,
		)
	}

	return jsonResponse.Session.SID, nil
}

func checkPiholeSessionIDIsValid(instanceURL string, client *http.Client, sessionID string) (bool, error) {
	request, err := http.NewRequest("GET", instanceURL+"/api/auth", nil)
	if err != nil {
		return false, fmt.Errorf("creating session ID check request: %v", err)
	}
	request.Header.Set("x-ftl-sid", sessionID)

	response, err := client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusUnauthorized {
		return false, fmt.Errorf("session ID check request returned status %s", response.Status)
	}

	return response.StatusCode == http.StatusOK, nil
}

type technitiumStatsResponse struct {
	Response struct {
		Stats struct {
			TotalQueries   int `json:"totalQueries"`
			BlockedQueries int `json:"totalBlocked"`
			BlockedZones   int `json:"blockedZones"`
			BlockListZones int `json:"blockListZones"`
		} `json:"stats"`
		MainChartData struct {
			Datasets []struct {
				Label string `json:"label"`
				Data  []int  `json:"data"`
			} `json:"datasets"`
		} `json:"mainChartData"`
		TopBlockedDomains []struct {
			Domain string `json:"name"`
			Count  int    `json:"hits"`
		}
	} `json:"response"`
}

func fetchTechnitiumStats(instanceUrl string, allowInsecure bool, token string, noGraph bool) (*dnsStats, error) {
	if token == "" {
		return nil, errors.New("missing API token")
	}

	requestURL := strings.TrimRight(instanceUrl, "/") + "/api/dashboard/stats/get?token=" + token + "&type=LastDay"

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	var client requestDoer
	if !allowInsecure {
		client = defaultHTTPClient
	} else {
		client = defaultInsecureHTTPClient
	}

	responseJson, err := decodeJsonFromRequest[technitiumStatsResponse](client, request)
	if err != nil {
		return nil, err
	}

	var topBlockedDomainsCount = min(len(responseJson.Response.TopBlockedDomains), 5)

	stats := &dnsStats{
		TotalQueries:      responseJson.Response.Stats.TotalQueries,
		BlockedQueries:    responseJson.Response.Stats.BlockedQueries,
		TopBlockedDomains: make([]dnsStatsBlockedDomain, 0, topBlockedDomainsCount),
		DomainsBlocked:    responseJson.Response.Stats.BlockedZones + responseJson.Response.Stats.BlockListZones,
	}

	if stats.TotalQueries <= 0 {
		return stats, nil
	}

	stats.BlockedPercent = int(float64(responseJson.Response.Stats.BlockedQueries) / float64(responseJson.Response.Stats.TotalQueries) * 100)

	for i := 0; i < topBlockedDomainsCount; i++ {
		domain := responseJson.Response.TopBlockedDomains[i]
		firstDomain := domain.Domain

		if firstDomain == "" {
			continue
		}

		stats.TopBlockedDomains = append(stats.TopBlockedDomains, dnsStatsBlockedDomain{
			Domain: firstDomain,
		})

		if stats.BlockedQueries > 0 {
			stats.TopBlockedDomains[i].PercentBlocked = int(float64(domain.Count) / float64(responseJson.Response.Stats.BlockedQueries) * 100)
		}
	}

	if noGraph {
		return stats, nil
	}

	var queriesSeries, blockedSeries []int

	for _, label := range responseJson.Response.MainChartData.Datasets {
		switch label.Label {
		case "Total":
			queriesSeries = label.Data
		case "Blocked":
			blockedSeries = label.Data
		}
	}

	if len(queriesSeries) > dnsStatsHoursSpan {
		queriesSeries = queriesSeries[len(queriesSeries)-dnsStatsHoursSpan:]
	} else if len(queriesSeries) < dnsStatsHoursSpan {
		queriesSeries = append(make([]int, dnsStatsHoursSpan-len(queriesSeries)), queriesSeries...)
	}

	if len(blockedSeries) > dnsStatsHoursSpan {
		blockedSeries = blockedSeries[len(blockedSeries)-dnsStatsHoursSpan:]
	} else if len(blockedSeries) < dnsStatsHoursSpan {
		blockedSeries = append(make([]int, dnsStatsHoursSpan-len(blockedSeries)), blockedSeries...)
	}

	maxQueriesInSeries := 0

	for i := 0; i < dnsStatsBars; i++ {
		queries := 0
		blocked := 0

		for j := 0; j < dnsStatsHoursPerBar; j++ {
			queries += queriesSeries[i*dnsStatsHoursPerBar+j]
			blocked += blockedSeries[i*dnsStatsHoursPerBar+j]
		}

		stats.Series[i] = dnsStatsSeries{
			Queries: queries,
			Blocked: blocked,
		}

		if queries > 0 {
			stats.Series[i].PercentBlocked = int(float64(blocked) / float64(queries) * 100)
		}

		if queries > maxQueriesInSeries {
			maxQueriesInSeries = queries
		}
	}

	for i := 0; i < dnsStatsBars; i++ {
		stats.Series[i].PercentTotal = int(float64(stats.Series[i].Queries) / float64(maxQueriesInSeries) * 100)
	}

	return stats, nil
}
