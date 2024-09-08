package feed

import (
	"errors"
	"fmt"
	"log/slog"
)

type ReleaseSource string

const (
	ReleaseSourceCodeberg  ReleaseSource = "codeberg"
	ReleaseSourceGithub    ReleaseSource = "github"
	ReleaseSourceGitlab    ReleaseSource = "gitlab"
	ReleaseSourceDockerHub ReleaseSource = "dockerhub"
)

type ReleaseRequest struct {
	Source     ReleaseSource
	Repository string
	Token      *string
}

func FetchLatestReleases(requests []*ReleaseRequest) (AppReleases, error) {
	job := newJob(fetchLatestReleaseTask, requests).withWorkers(20)
	results, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	var failed int

	releases := make(AppReleases, 0, len(requests))

	for i := range results {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch release", "source", requests[i].Source, "repository", requests[i].Repository, "error", errs[i])
			continue
		}

		releases = append(releases, *results[i])
	}

	if failed == len(requests) {
		return nil, ErrNoContent
	}

	releases.SortByNewest()

	if failed > 0 {
		return releases, fmt.Errorf("%w: could not get %d releases", ErrPartialContent, failed)
	}

	return releases, nil
}

func fetchLatestReleaseTask(request *ReleaseRequest) (*AppRelease, error) {
	switch request.Source {
	case ReleaseSourceCodeberg:
		return fetchLatestCodebergRelease(request)
	case ReleaseSourceGithub:
		return fetchLatestGithubRelease(request)
	case ReleaseSourceGitlab:
		return fetchLatestGitLabRelease(request)
	case ReleaseSourceDockerHub:
		return fetchLatestDockerHubRelease(request)
	}

	return nil, errors.New("unsupported source")
}
