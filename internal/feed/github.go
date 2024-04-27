package feed

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type githubReleaseResponseJson struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HtmlUrl     string `json:"html_url"`
	Draft       bool   `json:"draft"`
	PreRelease  bool   `json:"prerelease"`
	Reactions   struct {
		Downvotes int `json:"-1"`
	} `json:"reactions"`
}

func parseGithubTime(t string) time.Time {
	parsedTime, err := time.Parse("2006-01-02T15:04:05Z", t)

	if err != nil {
		return time.Now()
	}

	return parsedTime
}

func FetchLatestReleasesFromGithub(repositories []string, token string) (AppReleases, error) {
	appReleases := make(AppReleases, 0, len(repositories))

	if len(repositories) == 0 {
		return appReleases, nil
	}

	requests := make([]*http.Request, len(repositories))

	for i, repository := range repositories {
		request, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=10", repository), nil)

		if token != "" {
			request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		}

		requests[i] = request
	}

	task := decodeJsonFromRequestTask[[]githubReleaseResponseJson](defaultClient)
	job := newJob(task, requests).withWorkers(15)
	responses, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch or parse github release", "error", errs[i], "url", requests[i].URL)
			continue
		}

		releases := responses[i]

		if len(releases) < 1 {
			failed++
			slog.Error("No releases found", "repository", repositories[i], "url", requests[i].URL)
			continue
		}

		var liveRelease *githubReleaseResponseJson

		for i := range releases {
			release := &releases[i]

			if !release.Draft && !release.PreRelease {
				liveRelease = release
				break
			}
		}

		if liveRelease == nil {
			slog.Error("No live release found", "repository", repositories[i], "url", requests[i].URL)
			continue
		}

		version := liveRelease.TagName

		if version[0] != 'v' {
			version = "v" + version
		}

		appReleases = append(appReleases, AppRelease{
			Name:         repositories[i],
			Version:      version,
			NotesUrl:     liveRelease.HtmlUrl,
			TimeReleased: parseGithubTime(liveRelease.PublishedAt),
			Downvotes:    liveRelease.Reactions.Downvotes,
		})
	}

	if len(appReleases) == 0 {
		return nil, ErrNoContent
	}

	appReleases.SortByNewest()

	if failed > 0 {
		return appReleases, fmt.Errorf("%w: could not get %d releases", ErrPartialContent, failed)
	}

	return appReleases, nil
}
