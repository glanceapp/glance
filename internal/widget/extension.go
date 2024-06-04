package widget

import (
	"context"
	"errors"
	"html/template"
	"net/url"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Extension struct {
	widgetBase `yaml:",inline"`
	URL        string            `yaml:"url"`
	Parameters map[string]string `yaml:"parameters"`
	AllowHtml  bool              `yaml:"allow-potentially-dangerous-html"`
	Extension  feed.Extension    `yaml:"-"`
	cachedHTML template.HTML     `yaml:"-"`
}

func (widget *Extension) Initialize() error {
	widget.withTitle("Extension").withCacheDuration(time.Minute * 30)

	if widget.URL == "" {
		return errors.New("no extension URL specified")
	}

	_, err := url.Parse(widget.URL)

	if err != nil {
		return err
	}

	return nil
}

func (widget *Extension) Update(ctx context.Context) {
	extension, err := feed.FetchExtension(feed.ExtensionRequestOptions{
		URL:        widget.URL,
		Parameters: widget.Parameters,
		AllowHtml:  widget.AllowHtml,
	})

	widget.canContinueUpdateAfterHandlingErr(err)

	widget.Extension = extension

	if extension.Title != "" {
		widget.Title = extension.Title
	}

	widget.cachedHTML = widget.render(widget, assets.ExtensionTemplate)
}

func (widget *Extension) Render() template.HTML {
	return widget.cachedHTML
}
