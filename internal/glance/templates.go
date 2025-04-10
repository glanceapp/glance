package glance

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var intl = message.NewPrinter(language.English)

var globalTemplateFunctions = template.FuncMap{
	"formatApproxNumber": formatApproxNumber,
	"formatNumber":       intl.Sprint,
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
	"formatPriceWithPrecision": func(precision int, price float64) string {
		return intl.Sprintf("%."+strconv.Itoa(precision)+"f", price)
	},
	"dynamicRelativeTimeAttrs": dynamicRelativeTimeAttrs,
	"formatBytes":              formatBytes,
	"formatServerMegabytes": func(mb uint64) template.HTML {
		formatted := formatBytes(mb * 1024 * 1024)
		parts := strings.Split(formatted, " ")
		if len(parts) == 2 {
			return template.HTML(parts[0] + ` <span class="color-base size-h5">` + parts[1] + `</span>`)
		}
		return template.HTML(formatted)
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

func formatApproxNumber(count int) string {
	if count < 1_000 {
		return strconv.Itoa(count)
	}

	if count < 10_000 {
		return strconv.FormatFloat(float64(count)/1_000, 'f', 1, 64) + "k"
	}

	if count < 1_000_000 {
		return strconv.Itoa(count/1_000) + "k"
	}

	return strconv.FormatFloat(float64(count)/1_000_000, 'f', 1, 64) + "m"
}

func dynamicRelativeTimeAttrs(t interface{ Unix() int64 }) template.HTMLAttr {
	return template.HTMLAttr(`data-dynamic-relative-time="` + strconv.FormatInt(t.Unix(), 10) + `"`)
}

func formatBytes(bytes uint64) string {
	type unitInfo struct {
		threshold uint64
		label     string
	}

	units := []unitInfo{
		{1 << 40, "TB"},
		{1 << 30, "GB"},
		{1 << 20, "MB"},
		{1 << 10, "KB"},
	}

	for _, u := range units {
		if bytes >= u.threshold {
			if bytes < 10*u.threshold {
				return fmt.Sprintf("%.1f %s", float64(bytes)/float64(u.threshold), u.label)
			}
			return fmt.Sprintf("%d %s", bytes/u.threshold, u.label)
		}
	}

	return fmt.Sprintf("%d B", bytes)
}
