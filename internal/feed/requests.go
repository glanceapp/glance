package feed

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const defaultClientTimeout = 5 * time.Second

var defaultClient = &http.Client{
	Timeout: defaultClientTimeout,
}

var insecureClientTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

var defaultInsecureClient = &http.Client{
	Timeout:   defaultClientTimeout,
	Transport: insecureClientTransport,
}

type RequestDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func addBrowserUserAgentHeader(request *http.Request) {
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0")
}

func truncateString(s string, maxLen int) string {
	asRunes := []rune(s)

	if len(asRunes) > maxLen {
		return string(asRunes[:maxLen])
	}

	return s
}

func decodeJsonFromRequest[T any](client RequestDoer, request *http.Request) (T, error) {
	response, err := client.Do(request)
	var result T

	if err != nil {
		return result, err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return result, err
	}

	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf(
			"unexpected status code %d for %s, response: %s",
			response.StatusCode,
			request.URL,
			truncateString(string(body), 256),
		)
	}

	err = json.Unmarshal(body, &result)

	if err != nil {
		return result, err
	}

	return result, nil
}

func decodeJsonFromRequestTask[T any](client RequestDoer) func(*http.Request) (T, error) {
	return func(request *http.Request) (T, error) {
		return decodeJsonFromRequest[T](client, request)
	}
}

// TODO: tidy up, these are a copy of the above but with a line changed
func decodeXmlFromRequest[T any](client RequestDoer, request *http.Request) (T, error) {
	response, err := client.Do(request)
	var result T

	if err != nil {
		return result, err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return result, err
	}

	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf(
			"unexpected status code %d for %s, response: %s",
			response.StatusCode,
			request.URL,
			truncateString(string(body), 256),
		)
	}

	err = xml.Unmarshal(body, &result)

	if err != nil {
		return result, err
	}

	return result, nil
}

func decodeXmlFromRequestTask[T any](client RequestDoer) func(*http.Request) (T, error) {
	return func(request *http.Request) (T, error) {
		return decodeXmlFromRequest[T](client, request)
	}
}

type workerPoolTask[I any, O any] struct {
	index  int
	input  I
	output O
	err    error
}

type workerPoolJob[I any, O any] struct {
	data    []I
	workers int
	task    func(I) (O, error)
	ctx     context.Context
}

const defaultNumWorkers = 10

func (job *workerPoolJob[I, O]) withWorkers(workers int) *workerPoolJob[I, O] {
	if workers == 0 {
		job.workers = defaultNumWorkers
	} else if workers > len(job.data) {
		job.workers = len(job.data)
	} else {
		job.workers = workers
	}

	return job
}

// func (job *workerPoolJob[I, O]) withContext(ctx context.Context) *workerPoolJob[I, O] {
// 	if ctx != nil {
// 		job.ctx = ctx
// 	}

// 	return job
// }

func newJob[I any, O any](task func(I) (O, error), data []I) *workerPoolJob[I, O] {
	return &workerPoolJob[I, O]{
		workers: defaultNumWorkers,
		task:    task,
		data:    data,
		ctx:     context.Background(),
	}
}

func workerPoolDo[I any, O any](job *workerPoolJob[I, O]) ([]O, []error, error) {
	results := make([]O, len(job.data))
	errs := make([]error, len(job.data))

	if len(job.data) == 0 {
		return results, errs, nil
	}

	tasksQueue := make(chan *workerPoolTask[I, O])
	resultsQueue := make(chan *workerPoolTask[I, O])

	var wg sync.WaitGroup

	for range job.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for t := range tasksQueue {
				t.output, t.err = job.task(t.input)
				resultsQueue <- t
			}
		}()
	}

	var err error

	go func() {
	loop:
		for i := range job.data {
			select {
			default:
				tasksQueue <- &workerPoolTask[I, O]{
					index: i,
					input: job.data[i],
				}
			case <-job.ctx.Done():
				err = job.ctx.Err()
				break loop
			}
		}

		close(tasksQueue)
		wg.Wait()
		close(resultsQueue)
	}()

	for task := range resultsQueue {
		errs[task.index] = task.err
		results[task.index] = task.output
	}

	return results, errs, err
}
