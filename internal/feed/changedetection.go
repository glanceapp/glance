package feed

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type changeDetectionResponseJson struct {
	Name        string `json:"title"`
	Url         string `json:"url"`
	LastChanged int    `json:"last_changed"`
}

	
func parseLastChangeTime(t int) time.Time {
	parsedTime := time.Unix(int64(t), 0)
	return parsedTime
}


func FetchLatestDetectedChanges(watches []string, token string) (ChangeWatches, error) {
	changeWatches := make(ChangeWatches, 0, len(watches))

	if len(watches) == 0 {
		return changeWatches, nil
	}

	requests := make([]*http.Request, len(watches))

	for i, repository := range watches {
		request, _ := http.NewRequest("GET", fmt.Sprintf("https://changedetection.knhash.in/api/v1/watch/%s", repository), nil)

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
			slog.Error("Failed to fetch or parse change detections", "error", errs[i], "url", requests[i].URL)
			continue
		}

		watch := responses[i]

		changeWatches = append(changeWatches, ChangeWatch{
			Name:        watch.Name,
			Url:         watch.Url,
			LastChanged: parseLastChangeTime(watch.LastChanged),
		})
	}

	if len(changeWatches) == 0 {
		return nil, ErrNoContent
	}

	changeWatches.SortByNewest()

	if failed > 0 {
		return changeWatches, fmt.Errorf("%w: could not get %d watches", ErrPartialContent, failed)
	}

	return changeWatches, nil
}
