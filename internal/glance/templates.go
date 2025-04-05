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

var intl = message.NewPrinter(language.English)

var globalTemplateFunctions = template.FuncMap{
	"formatViewerCount": formatViewerCount,
	"formatNumber":      intl.Sprint,
	"safeCSS": func(str string) template.CSS {
		return template.CSS(str)
	},
	"safeURL": func(str string) template.URL {
		return template.URL(str)
	},
	"absInt": func(i int) int {
		return int(math.Abs(float64(i)))
	},
	"formatPrice": func(price float64) string {
		return intl.Sprintf("%.2f", price)
	},
	"dynamicRelativeTimeAttrs": func(t time.Time) template.HTMLAttr {
		return template.HTMLAttr(`data-dynamic-relative-time="` + strconv.FormatInt(t.Unix(), 10) + `"`)
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
