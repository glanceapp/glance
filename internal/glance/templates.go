package glance

import (
	"fmt"
	"html/template"
	"math"
	"strconv"

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
	"safeHTML": func(str string) template.HTML {
		return template.HTML(str)
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
	"formatServerMegabytes": func(mb uint64) template.HTML {
		var value string
		var label string

		if mb < 1_000 {
			value = strconv.FormatUint(mb, 10)
			label = "MB"
		} else if mb < 1_000_000 {
			if mb < 10_000 {
				value = fmt.Sprintf("%.1f", float64(mb)/1_000)
			} else {
				value = strconv.FormatUint(mb/1_000, 10)
			}

			label = "GB"
		} else {
			value = fmt.Sprintf("%.1f", float64(mb)/1_000_000)
			label = "TB"
		}

		return template.HTML(value + ` <span class="color-base size-h5">` + label + `</span>`)
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
