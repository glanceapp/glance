package feed

import (
	"net/http"
	"strings"
)

type adguardStatsResponse struct {
	TotalQueries      int              `json:"num_dns_queries"`
	QueriesSeries     []int            `json:"dns_queries"`
	BlockedQueries    int              `json:"num_blocked_filtering"`
	BlockedSeries     []int            `json:"blocked_filtering"`
	ResponseTime      float64          `json:"avg_processing_time"`
	TopBlockedDomains []map[string]int `json:"top_blocked_domains"`
}

func FetchAdguardStats(instanceURL, username, password string) (*DNSStats, error) {
	requestURL := strings.TrimRight(instanceURL, "/") + "/control/stats"

	request, err := http.NewRequest("GET", requestURL, nil)

	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(username, password)

	responseJson, err := decodeJsonFromRequest[adguardStatsResponse](defaultClient, request)

	if err != nil {
		return nil, err
	}

	var topBlockedDomainsCount = min(len(responseJson.TopBlockedDomains), 5)

	stats := &DNSStats{
		TotalQueries:      responseJson.TotalQueries,
		BlockedQueries:    responseJson.BlockedQueries,
		ResponseTime:      int(responseJson.ResponseTime * 1000),
		TopBlockedDomains: make([]DNSStatsBlockedDomain, 0, topBlockedDomainsCount),
	}

	if stats.TotalQueries <= 0 {
		return stats, nil
	}

	stats.BlockedPercent = int(float64(responseJson.BlockedQueries) / float64(responseJson.TotalQueries) * 100)

	for i := 0; i < topBlockedDomainsCount; i++ {
		domain := responseJson.TopBlockedDomains[i]
		var firstDomain string

		for k := range domain {
			firstDomain = k
			break
		}

		if firstDomain == "" {
			continue
		}

		stats.TopBlockedDomains = append(stats.TopBlockedDomains, DNSStatsBlockedDomain{
			Domain: firstDomain,
		})

		if stats.BlockedQueries > 0 {
			stats.TopBlockedDomains[i].PercentBlocked = int(float64(domain[firstDomain]) / float64(responseJson.BlockedQueries) * 100)
		}
	}

	queriesSeries := responseJson.QueriesSeries
	blockedSeries := responseJson.BlockedSeries

	const bars = 8
	const hoursSpan = 24
	const hoursPerBar int = hoursSpan / bars

	if len(queriesSeries) > hoursSpan {
		queriesSeries = queriesSeries[len(queriesSeries)-hoursSpan:]
	} else if len(queriesSeries) < hoursSpan {
		queriesSeries = append(make([]int, hoursSpan-len(queriesSeries)), queriesSeries...)
	}

	if len(blockedSeries) > hoursSpan {
		blockedSeries = blockedSeries[len(blockedSeries)-hoursSpan:]
	} else if len(blockedSeries) < hoursSpan {
		blockedSeries = append(make([]int, hoursSpan-len(blockedSeries)), blockedSeries...)
	}

	maxQueriesInSeries := 0

	for i := 0; i < bars; i++ {
		queries := 0
		blocked := 0

		for j := 0; j < hoursPerBar; j++ {
			queries += queriesSeries[i*hoursPerBar+j]
			blocked += blockedSeries[i*hoursPerBar+j]
		}

		stats.Series[i] = DNSStatsSeries{
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

	for i := 0; i < bars; i++ {
		stats.Series[i].PercentTotal = int(float64(stats.Series[i].Queries) / float64(maxQueriesInSeries) * 100)
	}

	return stats, nil
}
