package glance

import (
	"context"
	"html/template"
)

type faqWidget struct {
	widgetBase
	Questions []FAQItem `yaml:"questions"`
}

type FAQItem struct {
	Question string `yaml:"question"`
	Answer   string `yaml:"answer"`
}

var faqTemplate = mustParseTemplate("faq.html")

func (w *faqWidget) initialize() error {
	w.Type = "faq"
	w.withCacheType(cacheTypeInfinite)
	return nil
}

func (w *faqWidget) update(ctx context.Context) {
	w.ContentAvailable = len(w.Questions) > 0
}

func (w *faqWidget) Render() template.HTML {
	return w.renderTemplate(w, faqTemplate)
}

func (w *faqWidget) withCacheType(cType cacheType) *faqWidget {
	w.cacheType = cType
	return w
}
