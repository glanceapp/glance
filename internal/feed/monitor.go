package feed

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type SiteStatusRequest struct {
	URL           string `yaml:"url"`
	AllowInsecure bool   `yaml:"allow-insecure"`
}

type SiteStatus struct {
	Code         int
	TimedOut     bool
	ResponseTime time.Duration
	Error        error
}

func getSiteStatusTask(statusRequest *SiteStatusRequest) (SiteStatus, error) {
	request, err := http.NewRequest(http.MethodGet, statusRequest.URL, nil)

	if err != nil {
		return SiteStatus{
			Error: err,
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	request = request.WithContext(ctx)
	requestSentAt := time.Now()
	var response *http.Response

	if !statusRequest.AllowInsecure {
		response, err = defaultClient.Do(request)
	} else {
		response, err = defaultInsecureClient.Do(request)
	}

	status := SiteStatus{ResponseTime: time.Since(requestSentAt)}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			status.TimedOut = true
		}

		status.Error = err
		return status, nil
	}

	defer response.Body.Close()

	status.Code = response.StatusCode

	return status, nil
}

func FetchStatusForSites(requests []*SiteStatusRequest) ([]SiteStatus, error) {
	job := newJob(getSiteStatusTask, requests).withWorkers(20)
	results, _, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	return results, nil
}
