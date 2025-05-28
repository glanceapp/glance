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

type SearchShortcut struct {
	Title    string `yaml:"title"`
	URL      string `yaml:"url"`
	Shortcut string `yaml:"shortcut"`
}

type searchWidget struct {
	widgetBase       `yaml:",inline"`
	cachedHTML       template.HTML    `yaml:"-"`
	SearchEngine     string           `yaml:"search-engine"`
	Bangs            []SearchBang     `yaml:"bangs"`
	Shortcuts        []SearchShortcut `yaml:"shortcuts"`
	Suggestions      bool             `yaml:"suggestions"`
	SuggestionEngine string           `yaml:"suggestion-engine"`
	NewTab           bool             `yaml:"new-tab"`
	Target           string           `yaml:"target"`
	Autofocus        bool             `yaml:"autofocus"`
	Placeholder      string           `yaml:"placeholder"`
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
	"kagi":       "https://kagi.com/search?q={QUERY}",
	"startpage": "https://www.startpage.com/search?q={QUERY}",
}

var suggestionEngines = map[string]string{
	"google":     "https://suggestqueries.google.com/complete/search?output=firefox&q={QUERY}",
	"duckduckgo": "https://duckduckgo.com/ac/?q={QUERY}&type=list",
	"bing":       "https://www.bing.com/osjson.aspx?query={QUERY}",
	"startpage":  "https://startpage.com/suggestions?q={QUERY}&format=opensearch",
}

func (widget *searchWidget) initialize() error {
	widget.withTitle("Search").withError(nil)

	if widget.SearchEngine == "" {
		widget.SearchEngine = "duckduckgo"
	}

	if widget.Placeholder == "" {
		widget.Placeholder = "Type here to search…"
	}

	originalSearchEngine := widget.SearchEngine
	if url, ok := searchEngines[widget.SearchEngine]; ok {
		widget.SearchEngine = url
	}

	widget.SearchEngine = convertSearchUrl(widget.SearchEngine)

	// Set default suggestion engine if suggestions are enabled
	if widget.Suggestions && widget.SuggestionEngine == "" {
		widget.SuggestionEngine = originalSearchEngine
	}

	// Convert suggestion engine preset to URL if applicable
	if widget.Suggestions && widget.SuggestionEngine != "" {
		if url, ok := suggestionEngines[widget.SuggestionEngine]; ok {
			widget.SuggestionEngine = url
		}
	}

	for i := range widget.Bangs {
		if widget.Bangs[i].Shortcut == "" {
			return fmt.Errorf("search bang #%d has no shortcut", i+1)
		}

		if widget.Bangs[i].URL == "" {
			return fmt.Errorf("search bang #%d has no URL", i+1)
		}

		widget.Bangs[i].URL = convertSearchUrl(widget.Bangs[i].URL)
	}

	for i := range widget.Shortcuts {
		if widget.Shortcuts[i].Title == "" {
			return fmt.Errorf("search shortcut #%d has no title", i+1)
		}

		if widget.Shortcuts[i].URL == "" {
			return fmt.Errorf("search shortcut #%d has no URL", i+1)
		}
	}

	widget.ContentAvailable = true
	widget.cachedHTML = widget.renderTemplate(widget, searchWidgetTemplate)
	return nil
}

func (widget *searchWidget) Render() template.HTML {
	return widget.cachedHTML
}
