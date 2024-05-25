package feed

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SonarrConfig struct {
	Enable   bool   `yaml:"enable"`
	Endpoint string `yaml:"endpoint"`
	ApiKey   string `yaml:"apikey"`
}

type RadarrConfig struct {
	Enable   bool   `yaml:"enable"`
	Endpoint string `yaml:"endpoint"`
	ApiKey   string `yaml:"apikey"`
}

type ArrRelease struct {
	Title         string
	ImageCoverUrl string
	AirDateUtc    string
	SeasonNumber  *string
	EpisodeNumber *string
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

type RadarrReleaseResponse struct {
	HasFile bool   `json:"hasFile"`
	Title   string `json:"title"`
	Images  []struct {
		CoverType string `json:"coverType"`
		RemoteUrl string `json:"remoteUrl"`
	} `json:"images"`
	InCinemasDate       string `json:"inCinemas"`
	PhysicalReleaseDate string `json:"physicalRelease"`
	DigitalReleaseDate  string `json:"digitalRelease"`
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

		airDate, err := time.Parse(time.RFC3339, release.AirDateUtc)
		if err != nil {
			return nil, fmt.Errorf("failed to parse air date: %v", err)
		}

		// Format the date as YYYY-MM-DD HH:MM:SS
		formattedDate := airDate.Format("2006-01-02 15:04:05")

		// Format SeasonNumber and EpisodeNumber with at least two digits
		seasonNumber := fmt.Sprintf("%02d", release.SeasonNumber)
		episodeNumber := fmt.Sprintf("%02d", release.EpisodeNumber)

		releases = append(releases, ArrRelease{
			Title:         release.Series.Title,
			ImageCoverUrl: imageCover,
			AirDateUtc:    formattedDate,
			SeasonNumber:  &seasonNumber,
			EpisodeNumber: &episodeNumber,
			Grabbed:       release.HasFile,
		})
	}

	return releases, nil
}

func FetchReleasesFromRadarr(RadarrEndpoint string, RadarrApiKey string) (ArrReleases, error) {
	if RadarrEndpoint == "" {
		return nil, fmt.Errorf("missing radarr-endpoint config")
	}

	if RadarrApiKey == "" {
		return nil, fmt.Errorf("missing radarr-apikey config")
	}

	client := &http.Client{}
	url := fmt.Sprintf("%s/api/v3/calendar", strings.TrimSuffix(RadarrEndpoint, "/"))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-Api-Key", RadarrApiKey)
	req.Header.Set("Host", extractHostFromURL(RadarrEndpoint))
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

	var radarrReleases []RadarrReleaseResponse
	err = json.Unmarshal(body, &radarrReleases)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	var releases ArrReleases
	for _, release := range radarrReleases {
		var imageCover string
		for _, image := range release.Images {
			if image.CoverType == "poster" {
				imageCover = image.RemoteUrl
				break
			}
		}

		// Choose the appropriate release date from Radarr's response
		releaseDate := release.InCinemasDate
		formattedDate := "In Cinemas: "
		if release.PhysicalReleaseDate != "" {
			releaseDate = release.PhysicalReleaseDate
			formattedDate = "Physical Release: "
		} else if release.DigitalReleaseDate != "" {
			releaseDate = release.DigitalReleaseDate
			formattedDate = "Digital Release: "
		}

		airDate, err := time.Parse("2006-01-02", releaseDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse release date: %v", err)
		}

		// Format the date as YYYY-MM-DD HH:MM:SS
		formattedDate = formattedDate + airDate.Format("2006-01-02 15:04:05")

		releases = append(releases, ArrRelease{
			Title:         release.Title,
			ImageCoverUrl: imageCover,
			AirDateUtc:    formattedDate,
			Grabbed:       release.HasFile,
		})
	}

	return releases, nil
}

func FetchReleasesFromArrStack(Sonarr SonarrConfig, Radarr RadarrConfig) (ArrReleases, error) {
	result := ArrReleases{}

	// Call FetchReleasesFromSonarr and handle the result
	if Sonarr.Enable {
		sonarrReleases, err := FetchReleasesFromSonarr(Sonarr.Endpoint, Sonarr.ApiKey)
		if err != nil {
			slog.Warn("failed to fetch release from sonarr", "error", err)
			return nil, err
		}

		result = append(result, sonarrReleases...)
	}

	// Call FetchReleasesFromRadarr and handle the result
	if Radarr.Enable {
		radarrReleases, err := FetchReleasesFromRadarr(Radarr.Endpoint, Radarr.ApiKey)
		if err != nil {
			slog.Warn("failed to fetch release from radarr", "error", err)
			return nil, err
		}

		result = append(result, radarrReleases...)
	}

	return result, nil
}
