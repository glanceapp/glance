package feed

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
)

type TwitchCategory struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	AvatarUrl    string `json:"avatarURL"`
	ViewersCount int    `json:"viewersCount"`
	Tags         []struct {
		Name string `json:"tagName"`
	} `json:"tags"`
	GameReleaseDate string `json:"originalReleaseDate"`
	IsNew           bool   `json:"-"`
}

type TwitchChannel struct {
	Login        string
	Exists       bool
	Name         string
	AvatarUrl    string
	IsLive       bool
	LiveSince    time.Time
	Category     string
	CategorySlug string
	ViewersCount int
}

type TwitchChannels []TwitchChannel

func (channels TwitchChannels) SortByViewers() {
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ViewersCount > channels[j].ViewersCount
	})
}

func (channels TwitchChannels) SortByLive() {
	sort.SliceStable(channels, func(i, j int) bool {
		return channels[i].IsLive && !channels[j].IsLive
	})
}

type twitchOperationResponse struct {
	Data       json.RawMessage
	Extensions struct {
		OperationName string `json:"operationName"`
	}
}

type twitchChannelShellOperationResponse struct {
	UserOrError struct {
		Type            string `json:"__typename"`
		DisplayName     string `json:"displayName"`
		ProfileImageUrl string `json:"profileImageURL"`
		Stream          *struct {
			ViewersCount int `json:"viewersCount"`
		}
	} `json:"userOrError"`
}

type twitchStreamMetadataOperationResponse struct {
	UserOrNull *struct {
		Stream *struct {
			StartedAt string `json:"createdAt"`
			Game      *struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"game"`
		} `json:"stream"`
	} `json:"user"`
}

type twitchDirectoriesOperationResponse struct {
	Data struct {
		DirectoriesWithTags struct {
			Edges []struct {
				Node TwitchCategory `json:"node"`
			} `json:"edges"`
		} `json:"directoriesWithTags"`
	} `json:"data"`
}

const twitchGqlEndpoint = "https://gql.twitch.tv/gql"
const twitchGqlClientId = "kimne78kx3ncx6brgo4mv6wki5h1ko"

const twitchDirectoriesOperationRequestBody = `[{"operationName": "BrowsePage_AllDirectories","variables": {"limit": %d,"options": {"sort": "VIEWER_COUNT","tags": []}},"extensions": {"persistedQuery": {"version": 1,"sha256Hash": "2f67f71ba89f3c0ed26a141ec00da1defecb2303595f5cda4298169549783d9e"}}}]`

func FetchTopGamesFromTwitch(exclude []string, limit int) ([]TwitchCategory, error) {
	reader := strings.NewReader(fmt.Sprintf(twitchDirectoriesOperationRequestBody, len(exclude)+limit))
	request, _ := http.NewRequest("POST", twitchGqlEndpoint, reader)
	request.Header.Add("Client-ID", twitchGqlClientId)
	response, err := decodeJsonFromRequest[[]twitchDirectoriesOperationResponse](defaultClient, request)

	if err != nil {
		return nil, err
	}

	if len(response) == 0 {
		return nil, errors.New("no categories could be retrieved")
	}

	edges := (response)[0].Data.DirectoriesWithTags.Edges
	categories := make([]TwitchCategory, 0, len(edges))

	for i := range edges {
		if slices.Contains(exclude, edges[i].Node.Slug) {
			continue
		}

		category := &edges[i].Node
		category.AvatarUrl = strings.Replace(category.AvatarUrl, "285x380", "144x192", 1)

		if len(category.Tags) > 2 {
			category.Tags = category.Tags[:2]
		}

		gameReleasedDate, err := time.Parse("2006-01-02T15:04:05Z", category.GameReleaseDate)

		if err == nil {
			if time.Since(gameReleasedDate) < 14*24*time.Hour {
				category.IsNew = true
			}
		}

		categories = append(categories, *category)
	}

	if len(categories) > limit {
		categories = categories[:limit]
	}

	return categories, nil
}

const twitchChannelStatusOperationRequestBody = `[{"operationName":"ChannelShell","variables":{"login":"%s"},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"580ab410bcd0c1ad194224957ae2241e5d252b2c5173d8e0cce9d32d5bb14efe"}}},{"operationName":"StreamMetadata","variables":{"channelLogin":"%s"},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"676ee2f834ede42eb4514cdb432b3134fefc12590080c9a2c9bb44a2a4a63266"}}}]`

// TODO: rework
// The operations for multiple channels can all be sent in a single request
// rather than sending a separate request for each channel. Need to figure out
// what the limit is for max operations per request and batch operations in
// multiple requests if number of channels exceeds allowed limit.

func fetchChannelFromTwitchTask(channel string) (TwitchChannel, error) {
	result := TwitchChannel{
		Login: strings.ToLower(channel),
	}

	reader := strings.NewReader(fmt.Sprintf(twitchChannelStatusOperationRequestBody, channel, channel))
	request, _ := http.NewRequest("POST", twitchGqlEndpoint, reader)
	request.Header.Add("Client-ID", twitchGqlClientId)

	response, err := decodeJsonFromRequest[[]twitchOperationResponse](defaultClient, request)

	if err != nil {
		return result, err
	}

	if len(response) != 2 {
		return result, fmt.Errorf("expected 2 operation responses, got %d", len(response))
	}

	var channelShell twitchChannelShellOperationResponse
	var streamMetadata twitchStreamMetadataOperationResponse

	for i := range response {
		switch response[i].Extensions.OperationName {
		case "ChannelShell":
			err = json.Unmarshal(response[i].Data, &channelShell)

			if err != nil {
				return result, fmt.Errorf("failed to unmarshal channel shell: %w", err)
			}
		case "StreamMetadata":
			err = json.Unmarshal(response[i].Data, &streamMetadata)

			if err != nil {
				return result, fmt.Errorf("failed to unmarshal stream metadata: %w", err)
			}
		default:
			return result, fmt.Errorf("unknown operation name: %s", response[i].Extensions.OperationName)
		}
	}

	if channelShell.UserOrError.Type != "User" {
		result.Name = result.Login
		return result, nil
	}

	result.Exists = true
	result.Name = channelShell.UserOrError.DisplayName
	result.AvatarUrl = channelShell.UserOrError.ProfileImageUrl

	if channelShell.UserOrError.Stream != nil {
		result.IsLive = true
		result.ViewersCount = channelShell.UserOrError.Stream.ViewersCount

		if streamMetadata.UserOrNull != nil && streamMetadata.UserOrNull.Stream != nil && streamMetadata.UserOrNull.Stream.Game != nil {
			result.Category = streamMetadata.UserOrNull.Stream.Game.Name
			result.CategorySlug = streamMetadata.UserOrNull.Stream.Game.Slug
			startedAt, err := time.Parse("2006-01-02T15:04:05Z", streamMetadata.UserOrNull.Stream.StartedAt)

			if err == nil {
				result.LiveSince = startedAt
			} else {
				slog.Warn("failed to parse twitch stream started at", "error", err, "started_at", streamMetadata.UserOrNull.Stream.StartedAt)
			}
		}
	}

	return result, nil
}

func FetchChannelsFromTwitch(channelLogins []string) (TwitchChannels, error) {
	result := make(TwitchChannels, 0, len(channelLogins))

	job := newJob(fetchChannelFromTwitchTask, channelLogins).withWorkers(10)
	channels, errs, err := workerPoolDo(job)

	if err != nil {
		return result, err
	}

	var failed int

	for i := range channels {
		if errs[i] != nil {
			failed++
			slog.Warn("failed to fetch twitch channel", "channel", channelLogins[i], "error", errs[i])
			continue
		}

		result = append(result, channels[i])
	}

	if failed == len(channelLogins) {
		return result, ErrNoContent
	}

	if failed > 0 {
		return result, fmt.Errorf("%w: failed to fetch %d channels", ErrPartialContent, failed)
	}

	return result, nil
}
