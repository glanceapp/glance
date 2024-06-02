package feed

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
)

type ChangeDetectionWatch struct {
	Title        string
	URL          string
	LastChanged  time.Time
	DiffURL      string
	PreviousHash string
}

type ChangeDetectionWatches []ChangeDetectionWatch

func (r ChangeDetectionWatches) SortByNewest() ChangeDetectionWatches {
	sort.Slice(r, func(i, j int) bool {
		return r[i].LastChanged.After(r[j].LastChanged)
	})

	return r
}

type changeDetectionResponseJson struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	LastChanged  int64  `json:"last_changed"`
	DateCreated  int64  `json:"date_created"`
	PreviousHash string `json:"previous_md5"`
}

func FetchWatchUUIDsFromChangeDetection(instanceURL string, token string) ([]string, error) {
	request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/watch", instanceURL), nil)

	if token != "" {
		request.Header.Add("x-api-key", token)
	}

	uuidsMap, err := decodeJsonFromRequest[map[string]struct{}](defaultClient, request)

	if err != nil {
		return nil, fmt.Errorf("could not fetch list of watch UUIDs: %v", err)
	}

	uuids := make([]string, 0, len(uuidsMap))

	for uuid := range uuidsMap {
		uuids = append(uuids, uuid)
	}

	return uuids, nil
}

func FetchWatchesFromChangeDetection(instanceURL string, requestedWatchIDs []string, token string) (ChangeDetectionWatches, error) {
	watches := make(ChangeDetectionWatches, 0, len(requestedWatchIDs))

	if len(requestedWatchIDs) == 0 {
		return watches, nil
	}

	requests := make([]*http.Request, len(requestedWatchIDs))

	for i, repository := range requestedWatchIDs {
		request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/watch/%s", instanceURL, repository), nil)

		if token != "" {
			request.Header.Add("x-api-key", token)
		}

		requests[i] = request
	}

	task := decodeJsonFromRequestTask[changeDetectionResponseJson](defaultClient)
	job := newJob(task, requests).withWorkers(15)
	responses, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch or parse change detection watch", "error", errs[i], "url", requests[i].URL)
			continue
		}

		watchJson := responses[i]

		watch := ChangeDetectionWatch{
			URL:     watchJson.URL,
			DiffURL: fmt.Sprintf("%s/diff/%s?from_version=%d", instanceURL, requestedWatchIDs[i], watchJson.LastChanged-1),
		}

		if watchJson.LastChanged == 0 {
			watch.LastChanged = time.Unix(watchJson.DateCreated, 0)
		} else {
			watch.LastChanged = time.Unix(watchJson.LastChanged, 0)
		}

		if watchJson.Title != "" {
			watch.Title = watchJson.Title
		} else {
			watch.Title = strings.TrimPrefix(strings.Trim(stripURLScheme(watchJson.URL), "/"), "www.")
		}

		if watchJson.PreviousHash != "" {
			var hashLength = 8

			if len(watchJson.PreviousHash) < hashLength {
				hashLength = len(watchJson.PreviousHash)
			}

			watch.PreviousHash = watchJson.PreviousHash[0:hashLength]
		}

		watches = append(watches, watch)
	}

	if len(watches) == 0 {
		return nil, ErrNoContent
	}

	watches.SortByNewest()

	if failed > 0 {
		return watches, fmt.Errorf("%w: could not get %d watches", ErrPartialContent, failed)
	}

	return watches, nil
}
