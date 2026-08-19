package glance

import (
	"fmt"
	"html/template"
	"strings"
)

var searchWidgetTemplate = mustParseTemplate("search.html", "widget-base.html")

type SearchBang struct {
	Title    string
	Shortcut string
	URL      string
}

type searchWidget struct {
	widgetBase   `yaml:",inline"`
	cachedHTML   template.HTML `yaml:"-"`
	SearchEngine string        `yaml:"search-engine"`
	Bangs        []SearchBang  `yaml:"bangs"`
	NewTab       bool          `yaml:"new-tab"`
	Target       string        `yaml:"target"`
	Autofocus    bool          `yaml:"autofocus"`
	Placeholder  string        `yaml:"placeholder"`
}

func convertSearchUrl(url string) string {
	// Go's template is being stubborn and continues to escape the curlies in the
	// URL regardless of what the type of the variable is so this is my way around it
	return strings.ReplaceAll(url, "{QUERY}", "!QUERY!")
}

var searchEngines = map[string]string{
	"duckduckgo": "https://duckduckgo.com/?q={QUERY}",
	"google":     "https://www.google.com/search?q={QUERY}",
	"bing":       "https://www.bing.com/search?q={QUERY}",
	"perplexity": "https://www.perplexity.ai/search?q={QUERY}",
	"kagi": "https://kagi.com/search?q={QUERY}",
	"startpage": "https://www.startpage.com/search?q={QUERY}",
}

func (widget *searchWidget) initialize() error {
	widget.withTitle("Search").withError(nil)

	if widget.SearchEngine == "" {
		widget.SearchEngine = "duckduckgo"
	}

	if widget.Placeholder == "" {
		widget.Placeholder = "Type here to search…"
	}

	if url, ok := searchEngines[widget.SearchEngine]; ok {
		widget.SearchEngine = url
	}

	widget.SearchEngine = convertSearchUrl(widget.SearchEngine)

	for i := range widget.Bangs {
		if widget.Bangs[i].Shortcut == "" {
			return fmt.Errorf("search bang #%d has no shortcut", i+1)
		}

		if widget.Bangs[i].URL == "" {
			return fmt.Errorf("search bang #%d has no URL", i+1)
		}

		widget.Bangs[i].URL = convertSearchUrl(widget.Bangs[i].URL)
	}

	widget.cachedHTML = widget.renderTemplate(widget, searchWidgetTemplate)
	return nil
}

func (widget *searchWidget) Render() template.HTML {
	return widget.cachedHTML
}
