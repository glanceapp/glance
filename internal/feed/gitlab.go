package feed

import (
	"fmt"
	"net/http"
	"net/url"
)

type gitlabReleaseResponseJson struct {
	TagName    string `json:"tag_name"`
	ReleasedAt string `json:"released_at"`
	Links      struct {
		Self string `json:"self"`
	} `json:"_links"`
}

func fetchLatestGitLabRelease(request *ReleaseRequest) (*AppRelease, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://gitlab.com/api/v4/projects/%s/releases/permalink/latest",
			url.QueryEscape(request.Repository),
		),
		nil,
	)

	if err != nil {
		return nil, err
	}

	if request.Token != nil {
		httpRequest.Header.Add("PRIVATE-TOKEN", *request.Token)
	}

	response, err := decodeJsonFromRequest[gitlabReleaseResponseJson](defaultClient, httpRequest)

	if err != nil {
		return nil, err
	}

	return &AppRelease{
		Source:       ReleaseSourceGitlab,
		Name:         request.Repository,
		Version:      normalizeVersionFormat(response.TagName),
		NotesUrl:     response.Links.Self,
		TimeReleased: parseRFC3339Time(response.ReleasedAt),
	}, nil
}
