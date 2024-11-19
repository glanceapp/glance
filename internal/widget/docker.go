package widget

import (
	"context"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type containerData struct {
	Id          string
	Image       string
	URL         string
	Title       string
	Icon        CustomIcon
	StatusShort string
	StatusFull  string
	StatusStyle string
}

type Docker struct {
	widgetBase `yaml:",inline"`
	Containers []containerData `yaml:"-"`
}

func (widget *Docker) Initialize() error {
	widget.withTitle("Docker").withCacheDuration(1 * time.Minute)
	return nil
}

func (widget *Docker) Update(ctx context.Context) {
	containers, err := feed.FetchDockerContainers(ctx)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	var items []containerData
	for _, container := range containers {
		var item containerData
		item.Id = container.Id
		item.Image = container.Image
		item.URL = container.URL
		item.Title = container.Title

		_ = item.Icon.FromURL(container.IconURL)

		switch container.State {
		case "paused":
		case "starting":
		case "unhealthy":
			item.StatusStyle = "warning"
			break
		case "stopped":
		case "dead":
		case "exited":
			item.StatusStyle = "error"
			break
		default:
			item.StatusStyle = "success"
		}

		item.StatusFull = container.Status
		item.StatusShort = cases.Title(language.English, cases.Compact).String(container.State)

		items = append(items, item)
	}

	widget.Containers = items
}

func (widget *Docker) Render() template.HTML {
	return widget.render(widget, assets.DockerTemplate)
}
