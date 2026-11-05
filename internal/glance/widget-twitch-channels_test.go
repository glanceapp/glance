package glance

import (
	"strings"
	"testing"
)

func TestTwitchChannelsWidgetSameTab(t *testing.T) {
	widget := &twitchChannelsWidget{SameTab: true}
	widget.setChannels(twitchChannelList{{
		Login:        "channel",
		Exists:       true,
		Name:         "Channel",
		IsLive:       true,
		Category:     "Game",
		CategorySlug: "game",
	}})
	if err := widget.initialize(); err != nil {
		t.Fatalf("initialize Twitch channels widget: %v", err)
	}
	widget.withError(nil)

	rendered := string(widget.Render())
	if strings.Contains(rendered, `href="https://twitch.tv/channel" target="_blank"`) {
		t.Fatal("expected channel links to open in the same tab")
	}
	if !strings.Contains(rendered, `href="https://www.twitch.tv/directory/category/game" target="_blank"`) {
		t.Fatalf("expected category links to keep opening in a new tab:\n%s", rendered)
	}
}
