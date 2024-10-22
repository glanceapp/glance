package feed

import (
	"fmt"
	"net/http"
)

type codebergReleaseResponseJson struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HtmlUrl     string `json:"html_url"`
}

func fetchLatestCodebergRelease(request *ReleaseRequest) (*AppRelease, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://codeberg.org/api/v1/repos/%s/releases/latest",
			request.Repository,
		),
		nil,
	)
	if err != nil {
		return nil, err
	}

	response, err := decodeJsonFromRequest[codebergReleaseResponseJson](defaultClient, httpRequest)

	if err != nil {
		return nil, err
	}
	return &AppRelease{
		Source:       ReleaseSourceCodeberg,
		Name:         request.Repository,
		Version:      normalizeVersionFormat(response.TagName),
		NotesUrl:     response.HtmlUrl,
		TimeReleased: parseRFC3339Time(response.PublishedAt),
	}, nil
}
