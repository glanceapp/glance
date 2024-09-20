package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type BGGHotness struct {
    widgetBase  `yaml:",inline"`
    Games       feed.BggBoardGames  `yaml:"`
    CollapseAfterRows int         `yaml:"collapse-after-rows"`
	Limit             int         `yaml:"limit"`
}

func (widget *BGGHotness) Initialize() error {
    widget.withTitle("BGG Hotness").withCacheDuration(time.Hour)

    if widget.Limit <= 0 {
        widget.Limit = 20
    }

    if widget.CollapseAfterRows == 0 || widget.CollapseAfterRows < -1 {
        widget.CollapseAfterRows = 4
    }
    return nil
}

func (widget *BGGHotness) Update(ctx context.Context) {
    games, err := feed.FetchBGGHotnessList()

    if !widget.canContinueUpdateAfterHandlingErr(err) {
        return
    }

    if len(games) > widget.Limit {
        games = games[:widget.Limit]
    }

    widget.Games = games
}

func (widget *BGGHotness) Render() template.HTML {
    
    return widget.render(widget, assets.BGGHotnessTemplate) 
}
