package feed

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

type SonarrConfig struct {
	Enable   bool   `yaml:"enable"`
	Endpoint string `yaml:"endpoint"`
	ApiKey   string `yaml:"apikey"`
}

type ArrRelease struct {
	Title         string
	ImageCoverUrl string
	AirDateUtc    string
	SeasonNumber  *int
	EpisodeNumber *int
	Grabbed       bool
}

type ArrReleases []ArrRelease

type SonarrReleaseResponse struct {
	HasFile       bool `json:"hasFile"`
	SeasonNumber  int  `json:"seasonNumber"`
	EpisodeNumber int  `json:"episodeNumber"`
	Series        struct {
		Title  string `json:"title"`
		Images []struct {
			CoverType string `json:"coverType"`
			RemoteUrl string `json:"remoteUrl"`
		} `json:"images"`
	} `json:"series"`
	AirDateUtc string `json:"airDateUtc"`
}

func extractHostFromURL(apiEndpoint string) string {
	u, err := url.Parse(apiEndpoint)
	if err != nil {
		return "127.0.0.1"
	}
	return u.Host
}

func FetchReleasesFromSonarr(SonarrEndpoint string, SonarrApiKey string) (ArrReleases, error) {
	if SonarrEndpoint == "" {
		return nil, fmt.Errorf("missing sonarr-endpoint config")
	}

	if SonarrApiKey == "" {
		return nil, fmt.Errorf("missing sonarr-apikey config")
	}

	client := &http.Client{}
	url := fmt.Sprintf("%s/api/v3/calendar?includeSeries=true", strings.TrimSuffix(SonarrEndpoint, "/"))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-Api-Key", SonarrApiKey)
	req.Header.Set("Host", extractHostFromURL(SonarrEndpoint))
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var sonarrReleases []SonarrReleaseResponse
	err = json.Unmarshal(body, &sonarrReleases)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	var releases ArrReleases
	for _, release := range sonarrReleases {
		var imageCover string
		for _, image := range release.Series.Images {
			if image.CoverType == "poster" {
				imageCover = image.RemoteUrl
				break
			}
		}

		releases = append(releases, ArrRelease{
			Title:         release.Series.Title,
			ImageCoverUrl: imageCover,
			AirDateUtc:    release.AirDateUtc,
			SeasonNumber:  &release.SeasonNumber,
			EpisodeNumber: &release.EpisodeNumber,
			Grabbed:       release.HasFile,
		})
	}

	return releases, nil
}

func FetchReleasesFromArrStack(Sonarr SonarrConfig) (ArrReleases, error) {
	result := ArrReleases{}

	// Call FetchReleasesFromSonarr and handle the result
	if Sonarr.Enable {
		sonarrReleases, err := FetchReleasesFromSonarr(Sonarr.Endpoint, Sonarr.ApiKey)
		if err != nil {
			slog.Warn("failed to fetch release", "error", err)
			return nil, err
		}

		result = sonarrReleases
	}

	return result, nil
}
