package glance

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	pageTemplate        = mustParseTemplate("page.html", "document.html")
	pageContentTemplate = mustParseTemplate("page-content.html")
	forumPostsTemplate  = mustParseTemplate("forum-posts.html", "widget-base.html")
)

var globalTemplateFunctions = template.FuncMap{
	"formatViewerCount": formatViewerCount,
	"formatNumber":      intl.Sprint,
	"absInt": func(i int) int {
		return int(math.Abs(float64(i)))
	},
	"formatPrice": func(price float64) string {
		return intl.Sprintf("%.2f", price)
	},
	"dynamicRelativeTimeAttrs": func(t time.Time) template.HTMLAttr {
		return template.HTMLAttr(fmt.Sprintf(`data-dynamic-relative-time="%d"`, t.Unix()))
	},
}

func mustParseTemplate(primary string, dependencies ...string) *template.Template {
	t, err := template.New(primary).
		Funcs(globalTemplateFunctions).
		ParseFS(templateFS, append([]string{primary}, dependencies...)...)

	if err != nil {
		panic(err)
	}

	return t
}

var intl = message.NewPrinter(language.English)

func formatViewerCount(count int) string {
	if count < 1_000 {
		return strconv.Itoa(count)
	}

	if count < 10_000 {
		return fmt.Sprintf("%.1fk", float64(count)/1_000)
	}

	if count < 1_000_000 {
		return fmt.Sprintf("%dk", count/1_000)
	}

	return fmt.Sprintf("%.1fm", float64(count)/1_000_000)
}
