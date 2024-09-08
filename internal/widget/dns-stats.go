package widget

import (
	"context"
	"errors"
	"html/template"
	"strings"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type DNSStats struct {
	widgetBase `yaml:",inline"`

	TimeLabels [8]string      `yaml:"-"`
	Stats      *feed.DNSStats `yaml:"-"`

	HourFormat string            `yaml:"hour-format"`
	Service    string            `yaml:"service"`
	URL        OptionalEnvString `yaml:"url"`
	Token      OptionalEnvString `yaml:"token"`
	Username   OptionalEnvString `yaml:"username"`
	Password   OptionalEnvString `yaml:"password"`
}

func makeDNSTimeLabels(format string) [8]string {
	now := time.Now()
	var labels [8]string

	for i := 24; i > 0; i -= 3 {
		labels[7-(i/3-1)] = strings.ToLower(now.Add(-time.Duration(i) * time.Hour).Format(format))
	}

	return labels
}

func (widget *DNSStats) Initialize() error {
	widget.
		withTitle("DNS Stats").
		withTitleURL(string(widget.URL)).
		withCacheDuration(10 * time.Minute)

	if widget.Service != "adguard" && widget.Service != "pihole" {
		return errors.New("DNS stats service must be either 'adguard' or 'pihole'")
	}

	return nil
}

func (widget *DNSStats) Update(ctx context.Context) {
	var stats *feed.DNSStats
	var err error

	if widget.Service == "adguard" {
		stats, err = feed.FetchAdguardStats(string(widget.URL), string(widget.Username), string(widget.Password))
	} else {
		stats, err = feed.FetchPiholeStats(string(widget.URL), string(widget.Token))
	}

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.HourFormat == "24h" {
		widget.TimeLabels = makeDNSTimeLabels("15:00")
	} else {
		widget.TimeLabels = makeDNSTimeLabels("3PM")
	}

	widget.Stats = stats
}

func (widget *DNSStats) Render() template.HTML {
	return widget.render(widget, assets.DNSStatsTemplate)
}
