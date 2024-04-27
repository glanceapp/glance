package feed

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type SiteStatus struct {
	Code         int
	TimedOut     bool
	ResponseTime time.Duration
	Error        error
}

func getSiteStatusTask(request *http.Request) (SiteStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	request = request.WithContext(ctx)
	start := time.Now()
	response, err := http.DefaultClient.Do(request)
	took := time.Since(start)
	status := SiteStatus{ResponseTime: took}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			status.TimedOut = true
		}

		status.Error = err
		return status, err
	}

	defer response.Body.Close()

	status.Code = response.StatusCode

	return status, nil
}

func FetchStatusesForRequests(requests []*http.Request) ([]SiteStatus, error) {
	job := newJob(getSiteStatusTask, requests).withWorkers(20)
	results, _, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	return results, nil
}
