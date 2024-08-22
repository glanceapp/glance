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

	stats := &DNSStats{
		TotalQueries:   responseJson.TotalQueries,
		BlockedQueries: responseJson.BlockedQueries,
		ResponseTime:   int(responseJson.ResponseTime * 1000),
	}

	if stats.TotalQueries <= 0 {
		return stats, nil
	}

	stats.BlockedPercent = int(float64(responseJson.BlockedQueries) / float64(responseJson.TotalQueries) * 100)

	var topBlockedDomainsCount = min(len(responseJson.TopBlockedDomains), 5)

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
			Domain:         firstDomain,
			PercentBlocked: int(float64(domain[firstDomain]) / float64(responseJson.BlockedQueries) * 100),
		})
	}

	// Adguard _should_ return data for the last 24 hours in a 1 hour interval
	if len(responseJson.QueriesSeries) != 24 || len(responseJson.BlockedSeries) != 24 {
		return stats, nil
	}

	maxQueriesInSeries := 0

	for i := 0; i < 8; i++ {
		queries := 0
		blocked := 0

		for j := 0; j < 3; j++ {
			queries += responseJson.QueriesSeries[i*3+j]
			blocked += responseJson.BlockedSeries[i*3+j]
		}

		stats.Series[i] = DNSStatsSeries{
			Queries:        queries,
			Blocked:        blocked,
			PercentBlocked: int(float64(blocked) / float64(queries) * 100),
		}

		if queries > maxQueriesInSeries {
			maxQueriesInSeries = queries
		}
	}

	for i := 0; i < 8; i++ {
		stats.Series[i].PercentTotal = int(float64(stats.Series[i].Queries) / float64(maxQueriesInSeries) * 100)
	}

	return stats, nil
}
