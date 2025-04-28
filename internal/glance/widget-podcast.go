package glance

import (
	"context"
	"fmt"
	"github.com/samber/lo"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/antchfx/htmlquery"
)

type podcastChannel struct {
	podcastID string `yaml:"podcast_id"` // apple podcast id
	region    string `yaml:"-"`
	// podcastNameAlias string `yaml:"podcast_name"` // apple podcast id
}

type podcastChannelXML struct {
	Head struct {
		Script []struct {
			// ID          *string `xml:"id,attr"`
			PodcastJson string `xml:",chardata"`
		} `xml:"script"`
	} `xml:"head"`
}

type podcastChannelInfo struct {
	PodcastName           string
	PodcastEpisodeName    string
	PodcastEpisodeURL     string
	PodcastEpisodeIconURL string
	PodcastEpisodeSummary string
	PodcastEpisodeDate    time.Time
}

type podcastWidget struct {
	widgetBase       `yaml:",inline"`
	podcastChannels  []podcastChannel `yaml:"channels"`
	Style            string           `yaml:"style"`
	ThumbnailHeight  float64          `yaml:"thumbnail-height"`
	CardHeight       float64          `yaml:"card-height"`
	Items            rssFeedItemList  `yaml:"-"`
	Limit            int              `yaml:"limit"`
	CollapseAfter    int              `yaml:"collapse-after"`
	SingleLineTitles bool             `yaml:"single-line-titles"`
	PreserveOrder    bool             `yaml:"preserve-order"`
	NoItemsMessage   string           `yaml:"-"`
	Region           string           `yaml:"region"`
}

func (widget *podcastWidget) initialize() error {
	// widget.withTitle("RSS Feed").withCacheDuration(1 * time.Hour)

	// if widget.Limit <= 0 {
	// 	widget.Limit = 25
	// }

	// if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
	// 	widget.CollapseAfter = 5
	// }

	// if widget.ThumbnailHeight < 0 {
	// 	widget.ThumbnailHeight = 0
	// }

	// if widget.CardHeight < 0 {
	// 	widget.CardHeight = 0
	// }

	// if widget.Style == "detailed-list" {
	// 	for i := range widget.FeedRequests {
	// 		widget.FeedRequests[i].IsDetailed = true
	// 	}
	// }

	// widget.NoItemsMessage = "No items were returned from the feeds."

	return nil
}

func (widget *podcastWidget) update(ctx context.Context) {
	// items, err := fetchItemsFromRSSFeeds(widget.FeedRequests)

	// if !widget.canContinueUpdateAfterHandlingErr(err) {
	// 	return
	// }

	// if !widget.PreserveOrder {
	// 	items.sortByNewest()
	// }

	// if len(items) > widget.Limit {
	// 	items = items[:widget.Limit]
	// }

	// widget.Items = items
}

func (widget *podcastWidget) Render() template.HTML {
	if widget.Style == "horizontal-cards" {
		return widget.renderTemplate(widget, rssWidgetHorizontalCardsTemplate)
	}

	if widget.Style == "horizontal-cards-2" {
		return widget.renderTemplate(widget, rssWidgetHorizontalCards2Template)
	}

	if widget.Style == "detailed-list" {
		return widget.renderTemplate(widget, rssWidgetDetailedListTemplate)
	}

	return widget.renderTemplate(widget, rssWidgetTemplate)
}

func fetchPodcastChannels(channels []*podcastChannel) ([]*podcastChannelInfo, error) {
	job := newJob(fetchPodcastChannel, channels).withWorkers(30)
	podcastChannelInfoGroups, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}
	if lo.CoalesceOrEmpty(errs...) != nil {
		return nil, fmt.Errorf("fetchPodcastChannel error, err: %v", errs)
	}
	podcastChannelInfos := lo.Flatten(podcastChannelInfoGroups)
	sort.Slice(podcastChannelInfos, func(i, j int) bool {
		iPubDate := podcastChannelInfos[i].PodcastEpisodeDate.Unix()
		jPubDate := podcastChannelInfos[j].PodcastEpisodeDate.Unix()
		if iPubDate == jPubDate {
			return podcastChannelInfos[i].PodcastEpisodeName < podcastChannelInfos[j].PodcastEpisodeName
		}
		return iPubDate > jPubDate
	})
	return podcastChannelInfos, nil
}

func fetchPodcastChannel(channel *podcastChannel) ([]*podcastChannelInfo, error) {
	const applePodcastURL = "https://podcasts.apple.com/%s/podcast/%s"
	requestURL := fmt.Sprintf(applePodcastURL, channel.region, channel.podcastID)
	fmt.Printf("requestURL: %+v", requestURL)
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		slog.Error("fetch podcast channel request error", "error", err, "url", requestURL)
		return nil, err
	}
	xmlQueryNode, err := getHtmlQueryFileFromRequest(defaultHTTPClient, request)
	if err != nil {
		slog.Error("fetch podcast channel request error", "error", err, "url", requestURL)
		return nil, err
	}
	schemaShowNodes := htmlquery.Find(xmlQueryNode, "//script[@id='schema:show']")
	if len(schemaShowNodes) == 0 || schemaShowNodes[0].FirstChild == nil {
		return nil, fmt.Errorf("failed to find schema show node")
	}
	podcastSchemaShowNode, err := Unmarshal[podcastSchemaShowNode](schemaShowNodes[0].FirstChild.Data)
	if err != nil {
		slog.Error("failed to unmarshal schema show node", "err", err)
		return nil, err
	}
	return lo.FilterMap(podcastSchemaShowNode.WorkExample, func(e *PodcastWorkExample, index int) (*podcastChannelInfo, bool) {
		pubDate, err := time.ParseInLocation(time.DateOnly, e.DatePublished, time.Local)
		if err != nil {
			slog.Error("parse episode date failed", "pub_date", e.DatePublished, "err", err)
			return nil, false
		}
		return &podcastChannelInfo{
			PodcastName:           podcastSchemaShowNode.Name,
			PodcastEpisodeName:    e.Name,
			PodcastEpisodeURL:     e.Url,
			PodcastEpisodeIconURL: e.ThumbnailUrl,
			PodcastEpisodeSummary: e.Duration,
			PodcastEpisodeDate:    pubDate,
		}, true
	}), nil
}

type podcastSchemaShowNode struct {
	Context      string    `json:"@context"`
	Type         string    `json:"@type"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Genre        []string  `json:"genre"`
	Url          string    `json:"url"`
	DateModified time.Time `json:"dateModified"`
	ThumbnailUrl string    `json:"thumbnailUrl"`
	Review       []struct {
		Type          string `json:"@type"`
		Author        string `json:"author"`
		DatePublished string `json:"datePublished"`
		Name          string `json:"name"`
		ReviewBody    string `json:"reviewBody"`
		ReviewRating  struct {
			Type        string `json:"@type"`
			RatingValue int    `json:"ratingValue"`
			BestRating  int    `json:"bestRating"`
			WorstRating int    `json:"worstRating"`
		} `json:"reviewRating"`
		ItemReviewed struct {
			Type        string   `json:"@type"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Genre       []string `json:"genre"`
			Url         string   `json:"url"`
			Offers      []struct {
				Type     string `json:"@type"`
				Category string `json:"category"`
				Price    int    `json:"price"`
			} `json:"offers"`
			DateModified time.Time `json:"dateModified"`
			ThumbnailUrl string    `json:"thumbnailUrl"`
		} `json:"itemReviewed"`
	} `json:"review"`
	WorkExample []*PodcastWorkExample `json:"workExample"`
}

type PodcastWorkExample struct {
	Type          string   `json:"@type"`
	DatePublished string   `json:"datePublished"`
	Description   string   `json:"description"`
	Duration      string   `json:"duration"`
	Genre         []string `json:"genre"`
	Name          string   `json:"name"`
	Offers        []struct {
		Type     string `json:"@type"`
		Category string `json:"category"`
		Price    int    `json:"price"`
	} `json:"offers"`
	RequiresSubscription string `json:"requiresSubscription"`
	UploadDate           string `json:"uploadDate"`
	Url                  string `json:"url"`
	ThumbnailUrl         string `json:"thumbnailUrl"`
}
