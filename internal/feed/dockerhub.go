package feed

import (
	"fmt"
	"net/http"
	"strings"
)

type dockerHubRepositoryTagsResponse struct {
	Results []struct {
		Name       string `json:"name"`
		LastPushed string `json:"tag_last_pushed"`
	} `json:"results"`
}

const dockerHubReleaseNotesURLFormat = "https://hub.docker.com/r/%s/tags?name=%s"

func fetchLatestDockerHubRelease(request *ReleaseRequest) (*AppRelease, error) {
	parts := strings.Split(request.Repository, "/")

	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository name: %s", request.Repository)
	}

	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://hub.docker.com/v2/namespaces/%s/repositories/%s/tags", parts[0], parts[1]),
		nil,
	)

	if err != nil {
		return nil, err
	}

	if request.Token != nil {
		httpRequest.Header.Add("Authorization", "Bearer "+(*request.Token))
	}

	response, err := decodeJsonFromRequest[dockerHubRepositoryTagsResponse](defaultClient, httpRequest)

	if err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, fmt.Errorf("no tags found for repository: %s", request.Repository)
	}

	tag := response.Results[0]

	return &AppRelease{
		Source:       ReleaseSourceDockerHub,
		NotesUrl:     fmt.Sprintf(dockerHubReleaseNotesURLFormat, request.Repository, tag.Name),
		Name:         request.Repository,
		Version:      tag.Name,
		TimeReleased: parseRFC3339Time(tag.LastPushed),
	}, nil
}
