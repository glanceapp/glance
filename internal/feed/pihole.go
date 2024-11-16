package feed

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

type piholeStatsResponse struct {
	TotalQueries      int                     `json:"dns_queries_today"`
	QueriesSeries     map[int64]int           `json:"domains_over_time"`
	BlockedQueries    int                     `json:"ads_blocked_today"`
	BlockedSeries     map[int64]int           `json:"ads_over_time"`
	BlockedPercentage float64                 `json:"ads_percentage_today"`
	TopBlockedDomains piholeTopBlockedDomains `json:"top_ads"`
	DomainsBlocked    int                     `json:"domains_being_blocked"`
}

// If user has some level of privacy enabled on Pihole, `json:"top_ads"` is an empty array
// Use custom unmarshal behavior to avoid not getting the rest of the valid data when unmarshalling
type piholeTopBlockedDomains map[string]int

func (p *piholeTopBlockedDomains) UnmarshalJSON(data []byte) error {
	// NOTE: do not change to piholeTopBlockedDomains type here or it will cause a stack overflow
	// because of the UnmarshalJSON method getting called recursively
	temp := make(map[string]int)

	err := json.Unmarshal(data, &temp)

	if err != nil {
		*p = make(piholeTopBlockedDomains)
	} else {
		*p = temp
	}

	return nil
}

func FetchPiholeStats(instanceURL, token string) (*DNSStats, error) {
	if token == "" {
		return nil, errors.New("missing API token")
	}

	requestURL := strings.TrimRight(instanceURL, "/") +
		"/admin/api.php?summaryRaw&topItems&overTimeData10mins&auth=" + token

	request, err := http.NewRequest("GET", requestURL, nil)

	if err != nil {
		return nil, err
	}

	responseJson, err := decodeJsonFromRequest[piholeStatsResponse](defaultClient, request)

	if err != nil {
		return nil, err
	}

	stats := &DNSStats{
		TotalQueries:   responseJson.TotalQueries,
		BlockedQueries: responseJson.BlockedQueries,
		BlockedPercent: int(responseJson.BlockedPercentage),
		DomainsBlocked: responseJson.DomainsBlocked,
	}

	if len(responseJson.TopBlockedDomains) > 0 {
		domains := make([]DNSStatsBlockedDomain, 0, len(responseJson.TopBlockedDomains))

		for domain, count := range responseJson.TopBlockedDomains {
			domains = append(domains, DNSStatsBlockedDomain{
				Domain:         domain,
				PercentBlocked: int(float64(count) / float64(responseJson.BlockedQueries) * 100),
			})
		}

		sort.Slice(domains, func(a, b int) bool {
			return domains[a].PercentBlocked > domains[b].PercentBlocked
		})

		stats.TopBlockedDomains = domains[:min(len(domains), 5)]
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

	for i := 0; i < 8; i++ {
		queries := 0
		blocked := 0

		for j := 0; j < 18; j++ {
			index := lowestTimestamp + int64(i*10800+j*600)

			queries += responseJson.QueriesSeries[index]
			blocked += responseJson.BlockedSeries[index]
		}

		if queries > maxQueriesInSeries {
			maxQueriesInSeries = queries
		}

		stats.Series[i] = DNSStatsSeries{
			Queries: queries,
			Blocked: blocked,
		}

		if queries > 0 {
			stats.Series[i].PercentBlocked = int(float64(blocked) / float64(queries) * 100)
		}
	}

	for i := 0; i < 8; i++ {
		stats.Series[i].PercentTotal = int(float64(stats.Series[i].Queries) / float64(maxQueriesInSeries) * 100)
	}

	return stats, nil
}
