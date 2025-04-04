package glance

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
)

var twitchChannelsWidgetTemplate = mustParseTemplate("twitch-channels.html", "widget-base.html")

type twitchChannelsWidget struct {
	widgetBase      `yaml:",inline"`
	ChannelsRequest []string        `yaml:"channels"`
	Channels        []twitchChannel `yaml:"-"`
	CollapseAfter   int             `yaml:"collapse-after"`
	SortBy          string          `yaml:"sort-by"`
}

func (widget *twitchChannelsWidget) initialize() error {
	widget.
		withTitle("Twitch Channels").
		withTitleURL("https://www.twitch.tv/directory/following").
		withCacheDuration(time.Minute * 10)

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.SortBy != "viewers" && widget.SortBy != "live" {
		widget.SortBy = "viewers"
	}

	return nil
}

func (widget *twitchChannelsWidget) update(ctx context.Context) {
	channels, err := fetchChannelsFromTwitch(widget.ChannelsRequest)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.SortBy == "viewers" {
		channels.sortByViewers()
	} else if widget.SortBy == "live" {
		channels.sortByLive()
	}

	widget.Channels = channels
}

func (widget *twitchChannelsWidget) Render() template.HTML {
	return widget.renderTemplate(widget, twitchChannelsWidgetTemplate)
}

type twitchChannel struct {
	Login        string
	Exists       bool
	Name         string
	StreamTitle  string
	AvatarUrl    string
	IsLive       bool
	LiveSince    time.Time
	Category     string
	CategorySlug string
	ViewersCount int
}

type twitchChannelList []twitchChannel

func (channels twitchChannelList) sortByViewers() {
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ViewersCount > channels[j].ViewersCount
	})
}

func (channels twitchChannelList) sortByLive() {
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
		LastBroadcast *struct {
			Title string `json:"title"`
		}
	} `json:"user"`
}

const twitchChannelStatusOperationRequestBody = `[
{"operationName":"ChannelShell","variables":{"login":"%s"},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"580ab410bcd0c1ad194224957ae2241e5d252b2c5173d8e0cce9d32d5bb14efe"}}},
{"operationName":"StreamMetadata","variables":{"channelLogin":"%s"},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"676ee2f834ede42eb4514cdb432b3134fefc12590080c9a2c9bb44a2a4a63266"}}}
]`

// TODO: rework
// The operations for multiple channels can all be sent in a single request
// rather than sending a separate request for each channel. Need to figure out
// what the limit is for max operations per request and batch operations in
// multiple requests if number of channels exceeds allowed limit.

func fetchChannelFromTwitchTask(channel string) (twitchChannel, error) {
	result := twitchChannel{
		Login: strings.ToLower(channel),
	}

	reader := strings.NewReader(fmt.Sprintf(twitchChannelStatusOperationRequestBody, channel, channel))
	request, _ := http.NewRequest("POST", twitchGqlEndpoint, reader)
	request.Header.Add("Client-ID", twitchGqlClientId)

	response, err := decodeJsonFromRequest[[]twitchOperationResponse](defaultHTTPClient, request)
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
			if err = json.Unmarshal(response[i].Data, &channelShell); err != nil {
				return result, fmt.Errorf("unmarshalling channel shell: %w", err)
			}
		case "StreamMetadata":
			if err = json.Unmarshal(response[i].Data, &streamMetadata); err != nil {
				return result, fmt.Errorf("unmarshalling stream metadata: %w", err)
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

		if streamMetadata.UserOrNull != nil && streamMetadata.UserOrNull.Stream != nil {
			if streamMetadata.UserOrNull.LastBroadcast != nil {
				result.StreamTitle = streamMetadata.UserOrNull.LastBroadcast.Title
			}

			if streamMetadata.UserOrNull.Stream.Game != nil {
				result.Category = streamMetadata.UserOrNull.Stream.Game.Name
				result.CategorySlug = streamMetadata.UserOrNull.Stream.Game.Slug
			}
			startedAt, err := time.Parse("2006-01-02T15:04:05Z", streamMetadata.UserOrNull.Stream.StartedAt)

			if err == nil {
				result.LiveSince = startedAt
			} else {
				slog.Warn("Failed to parse Twitch stream started at", "error", err, "started_at", streamMetadata.UserOrNull.Stream.StartedAt)
			}
		}
	} else {
		// This prevents live channels with 0 viewers from being
		// incorrectly sorted lower than offline channels
		result.ViewersCount = -1
	}

	return result, nil
}

func fetchChannelsFromTwitch(channelLogins []string) (twitchChannelList, error) {
	result := make(twitchChannelList, 0, len(channelLogins))

	job := newJob(fetchChannelFromTwitchTask, channelLogins).withWorkers(10)
	channels, errs, err := workerPoolDo(job)
	if err != nil {
		return result, err
	}

	var failed int

	for i := range channels {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch Twitch channel", "channel", channelLogins[i], "error", errs[i])
			continue
		}

		result = append(result, channels[i])
	}

	if failed == len(channelLogins) {
		return result, errNoContent
	}

	if failed > 0 {
		return result, fmt.Errorf("%w: failed to fetch %d channels", errPartialContent, failed)
	}

	return result, nil
}
