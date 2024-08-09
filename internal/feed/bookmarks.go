package feed

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type StatusPage struct {
	URL               string         `yaml:"url"`
	ShowIfOperational bool           `yaml:"show-if-operational" default:"false"`
	StatusPageInfo    StatusPageInfo `yaml:"-"`
}

type StatusPageInfo struct {
	StatusResponse StatusResponse `json:"status"`
	Error          error
}

type StatusResponse struct {
	Indicator   string `json:"indicator"`
	Description string `json:"description"`
}

const summaryEndpointPath = "/api/v2/summary.json"

func getStatusPageTask(statusRequest *StatusPage) (StatusPageInfo, error) {
	request, err := http.NewRequest(http.MethodGet, statusRequest.URL+summaryEndpointPath, nil)

	if err != nil {
		return StatusPageInfo{
			Error: err,
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	request = request.WithContext(ctx)

	var response *http.Response
	response, err = defaultClient.Do(request)

	var status StatusPageInfo
	if err != nil {
		status.Error = err
		return status, nil
	}

	err = json.NewDecoder(response.Body).Decode(&status)

	defer response.Body.Close()

	return status, nil
}

func FetchStatusPages(requests []*StatusPage) ([]StatusPageInfo, error) {
	job := newJob(getStatusPageTask, requests).withWorkers(20)
	results, _, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	return results, nil
}
