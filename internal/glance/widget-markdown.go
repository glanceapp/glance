package glance

import (
	"fmt"
	"html/template"
	"os"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

var markdownWidgetTemplate = mustParseTemplate("markdown.html", "widget-base.html")

type markdownWidget struct {
	widgetBase    `yaml:",inline"`
	CompiledHTML  template.HTML `yaml:"-"`
	Source        string        `yaml:"source"`
	File          string        `yaml:"file"`
}

func (widget *markdownWidget) initialize() error {
	widget.WIP = true

	if widget.Title == "" {
		widget.Title = "Markdown"
	}

	widget.withTitle(widget.Title).withError(nil)

	var content string

	if widget.File != "" {
		fileContent, err := os.ReadFile(widget.File)
		if err != nil {
			return fmt.Errorf("failed to read markdown file: %w", err)
		}
		content = string(fileContent)
	} else if widget.Source != "" {
		content = widget.Source
	} else {
		return fmt.Errorf("either 'source' or 'file' must be specified")
	}

	// Parse markdown with standard extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(content))

	// Render to HTML
	renderer := html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
	})

	widget.CompiledHTML = template.HTML(markdown.Render(doc, renderer))

	return nil
}

func (widget *markdownWidget) Render() template.HTML {
	return widget.renderTemplate(widget, markdownWidgetTemplate)
}
