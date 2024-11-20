package widget

import (
	"context"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"html/template"
	"strings"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

const (
	defaultDockerHost   = "unix:///var/run/docker.sock"
	dockerGlanceEnable  = "glance.enable"
	dockerGlanceTitle   = "glance.title"
	dockerGlanceUrl     = "glance.url"
	dockerGlanceIconUrl = "glance.iconUrl"
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
	HostURL    string          `yaml:"host-url"`
	Containers []containerData `yaml:"-"`
}

func (widget *Docker) Initialize() error {
	widget.withTitle("Docker").withCacheDuration(1 * time.Minute)
	return nil
}

func (widget *Docker) Update(_ context.Context) {
	if widget.HostURL == "" {
		widget.HostURL = defaultDockerHost
	}

	containers, err := feed.FetchDockerContainers(widget.HostURL)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	var items []containerData
	for _, c := range containers {
		isGlanceEnabled := getLabelValue(c.Labels, dockerGlanceEnable, "true")

		if isGlanceEnabled != "true" {
			continue
		}

		var item containerData
		item.Id = c.Id
		item.Image = c.Image
		item.Title = getLabelValue(c.Labels, dockerGlanceTitle, strings.Join(c.Names, ""))
		item.URL = getLabelValue(c.Labels, dockerGlanceUrl, "")

		_ = item.Icon.FromURL(getLabelValue(c.Labels, dockerGlanceIconUrl, "si:docker"))

		switch c.State {
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

		item.StatusFull = c.Status
		item.StatusShort = cases.Title(language.English, cases.Compact).String(c.State)

		items = append(items, item)
	}

	widget.Containers = items
}

func (widget *Docker) Render() template.HTML {
	return widget.render(widget, assets.DockerTemplate)
}

// getLabelValue get string value associated to a label.
func getLabelValue(labels map[string]string, labelName, defaultValue string) string {
	if value, ok := labels[labelName]; ok && len(value) > 0 {
		return value
	}
	return defaultValue
}
