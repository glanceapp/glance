package glance

import (
	"strings"
	"testing"
)

func TestVideosWidgetSameTab(t *testing.T) {
	for _, style := range []string{"horizontal-cards", "grid-cards", "vertical-list"} {
		t.Run(style, func(t *testing.T) {
			widget := &videosWidget{
				Style:   style,
				SameTab: true,
			}
			widget.setVideos(videoList{{
				Title:     "Video",
				Url:       "https://example.com/video",
				Author:    "Channel",
				AuthorUrl: "https://example.com/channel",
			}})
			if err := widget.initialize(); err != nil {
				t.Fatalf("initialize videos widget: %v", err)
			}

			rendered := string(widget.Render())
			if strings.Contains(rendered, `href="https://example.com/video" target="_blank"`) {
				t.Fatal("expected video link to open in the same tab")
			}
			if strings.Contains(rendered, `href="https://example.com/channel" target="_blank"`) {
				t.Fatal("expected channel link to open in the same tab")
			}
		})
	}
}
