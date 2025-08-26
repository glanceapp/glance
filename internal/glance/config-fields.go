package glance

import (
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var hslColorFieldPattern = regexp.MustCompile(`^(?:hsla?\()?([\d\.]+)(?: |,)+([\d\.]+)%?(?: |,)+([\d\.]+)%?\)?$`)
var inStringPropertyPattern = regexp.MustCompile(`(?m)([a-zA-Z]+)\[(.*?)\]`)

const (
	hslHueMax        = 360
	hslSaturationMax = 100
	hslLightnessMax  = 100
)

type hslColorField struct {
	H float64
	S float64
	L float64
}

func (c *hslColorField) String() string {
	return fmt.Sprintf("hsl(%.1f, %.1f%%, %.1f%%)", c.H, c.S, c.L)
}

func (c *hslColorField) ToHex() string {
	return hslToHex(c.H, c.S, c.L)
}

func (c1 *hslColorField) SameAs(c2 *hslColorField) bool {
	if c1 == nil && c2 == nil {
		return true
	}
	if c1 == nil || c2 == nil {
		return false
	}
	return c1.H == c2.H && c1.S == c2.S && c1.L == c2.L
}

func (c *hslColorField) UnmarshalYAML(node *yaml.Node) error {
	var value string
	errorLine := fmt.Sprintf("line %d:", node.Line)

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := hslColorFieldPattern.FindStringSubmatch(value)

	if len(matches) != 4 {
		return fmt.Errorf("%s invalid HSL color format: %s", errorLine, value)
	}

	hue, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return err
	}

	if hue > hslHueMax {
		return fmt.Errorf("%s HSL hue must be between 0 and %d", errorLine, hslHueMax)
	}

	saturation, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return err
	}

	if saturation > hslSaturationMax {
		return fmt.Errorf("%s HSL saturation must be between 0 and %d", errorLine, hslSaturationMax)
	}

	lightness, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return err
	}

	if lightness > hslLightnessMax {
		return fmt.Errorf("%s HSL lightness must be between 0 and %d", errorLine, hslLightnessMax)
	}

	c.H = hue
	c.S = saturation
	c.L = lightness

	return nil
}

var durationFieldPattern = regexp.MustCompile(`^(\d+)(s|m|h|d)$`)

type durationField time.Duration

func (d *durationField) UnmarshalYAML(node *yaml.Node) error {
	var value string
	errorLine := fmt.Sprintf("line %d:", node.Line)

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := durationFieldPattern.FindStringSubmatch(value)

	if len(matches) != 3 {
		return fmt.Errorf("%s invalid duration format for value `%s`", errorLine, value)
	}

	duration, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("%s invalid duration value: %s", errorLine, matches[1])
	}

	switch matches[2] {
	case "s":
		*d = durationField(time.Duration(duration) * time.Second)
	case "m":
		*d = durationField(time.Duration(duration) * time.Minute)
	case "h":
		*d = durationField(time.Duration(duration) * time.Hour)
	case "d":
		*d = durationField(time.Duration(duration) * 24 * time.Hour)
	}

	return nil
}

type customIconField struct {
	URL        template.URL
	Color      string
	AutoInvert bool
}

func (h *customIconField) Elem() template.HTML {
	return h.ElemWithClass("")
}

func (h *customIconField) ElemWithClass(class string) template.HTML {
	if h.AutoInvert && h.Color == "" {
		class = "flat-icon " + class
	}

	if h.Color != "" {
		return template.HTML(
			`<div class="icon colored-icon ` + class + `" style="--icon-color: ` + h.Color + `; --icon-url: url('` + string(h.URL) + `')"></div>`,
		)
	}

	return template.HTML(
		`<img class="icon ` + class + `" src="` + string(h.URL) + `" alt="" loading="lazy">`,
	)
}

func newCustomIconField(value string) customIconField {
	const autoInvertPrefix = "auto-invert "
	field := customIconField{}

	if strings.HasPrefix(value, autoInvertPrefix) {
		field.AutoInvert = true
		value = strings.TrimPrefix(value, autoInvertPrefix)
	}

	value, properties := parseInStringProperties(value)

	if color, ok := properties["color"]; ok {
		switch color {
		case "primary":
			color = "var(--color-primary)"
		case "positive":
			color = "var(--color-positive)"
		case "negative":
			color = "var(--color-negative)"
		case "base":
			color = "var(--color-text-base)"
		case "subdue":
			color = "var(--color-text-subdue)"
		}

		field.Color = color
	}

	prefix, icon, found := strings.Cut(value, ":")
	if !found {
		field.URL = template.URL(value)
		return field
	}

	basename, ext, found := strings.Cut(icon, ".")
	if !found {
		ext = "svg"
		basename = icon
	}

	if ext != "svg" && ext != "png" {
		ext = "svg"
	}

	switch prefix {
	case "si":
		field.AutoInvert = true
		field.URL = template.URL("https://cdn.jsdelivr.net/npm/simple-icons@latest/icons/" + basename + ".svg")
	case "di":
		field.URL = template.URL("https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/" + ext + "/" + basename + "." + ext)
	case "mdi":
		field.AutoInvert = true
		field.URL = template.URL("https://cdn.jsdelivr.net/npm/@mdi/svg@latest/svg/" + basename + ".svg")
	case "sh":
		field.URL = template.URL("https://cdn.jsdelivr.net/gh/selfhst/icons/" + ext + "/" + basename + "." + ext)
	case "hi":
		field.AutoInvert = true
		field.URL = template.URL("https://cdn.jsdelivr.net/npm/heroicons@latest/24/" + basename + ".svg")
	default:
		field.URL = template.URL(value)
	}

	return field
}

func (i *customIconField) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return err
	}

	*i = newCustomIconField(value)
	return nil
}

func parseInStringProperties(value string) (string, map[string]string) {
	properties := make(map[string]string)

	value = inStringPropertyPattern.ReplaceAllStringFunc(value, func(match string) string {
		matches := inStringPropertyPattern.FindStringSubmatch(match)
		if len(matches) != 3 {
			return ""
		}

		properties[matches[1]] = matches[2]

		return ""
	})

	return strings.TrimSpace(value), properties
}

type proxyOptionsField struct {
	URL           string        `yaml:"url"`
	AllowInsecure bool          `yaml:"allow-insecure"`
	Timeout       durationField `yaml:"timeout"`
	client        *http.Client  `yaml:"-"`
}

func (p *proxyOptionsField) UnmarshalYAML(node *yaml.Node) error {
	type proxyOptionsFieldAlias proxyOptionsField
	alias := (*proxyOptionsFieldAlias)(p)
	var proxyURL string

	if err := node.Decode(&proxyURL); err != nil {
		if err := node.Decode(alias); err != nil {
			return err
		}
	}

	if proxyURL == "" && p.URL == "" {
		return nil
	}

	if p.URL != "" {
		proxyURL = p.URL
	}

	parsedUrl, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("parsing proxy URL: %v", err)
	}

	var timeout = defaultClientTimeout
	if p.Timeout > 0 {
		timeout = time.Duration(p.Timeout)
	}

	p.client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(parsedUrl),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.AllowInsecure},
		},
	}

	return nil
}

type queryParametersField map[string][]string

func (q *queryParametersField) UnmarshalYAML(node *yaml.Node) error {
	var decoded map[string]any

	if err := node.Decode(&decoded); err != nil {
		return err
	}

	*q = make(queryParametersField)

	errorLine := fmt.Sprintf("line %d:", node.Line)

	// TODO: refactor the duplication in the switch cases if any more types get added
	for key, value := range decoded {
		switch v := value.(type) {
		case string:
			(*q)[key] = []string{v}
		case int, int8, int16, int32, int64, float32, float64:
			(*q)[key] = []string{fmt.Sprintf("%v", v)}
		case bool:
			(*q)[key] = []string{fmt.Sprintf("%t", v)}
		case []string:
			(*q)[key] = append((*q)[key], v...)
		case []any:
			for _, item := range v {
				switch item := item.(type) {
				case string:
					(*q)[key] = append((*q)[key], item)
				case int, int8, int16, int32, int64, float32, float64:
					(*q)[key] = append((*q)[key], fmt.Sprintf("%v", item))
				case bool:
					(*q)[key] = append((*q)[key], fmt.Sprintf("%t", item))
				default:
					return fmt.Errorf("%s invalid query parameter value type: %T", errorLine, item)
				}
			}
		default:
			return fmt.Errorf("%s invalid query parameter value type: %T", errorLine, value)
		}
	}

	return nil
}

type sortKey struct {
	name string
	dir  int // 1 for asc, -1 for desc
}

func (q *queryParametersField) toQueryString() string {
	query := url.Values{}

	for key, values := range *q {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	return query.Encode()
}

type sortableFields[T any] struct {
	keys   []sortKey
	fields map[string]func(a, b T) int
}

func (s *sortableFields[T]) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return errors.New("sort-by must be a string")
	}

	return s.parse(raw)
}

func (s *sortableFields[T]) parse(raw string) error {
	split := strings.Split(raw, ",")

	for i := range split {
		key := strings.TrimSpace(split[i])
		direction := "asc"
		name, dir, ok := strings.Cut(key, ":")
		if ok {
			key = strings.TrimSpace(name)
			direction = strings.TrimSpace(dir)
			if direction != "asc" && direction != "desc" {
				return fmt.Errorf("unsupported sort direction `%s` in sort-by option `%s`, must be `asc` or `desc`", direction, key)
			}
		}

		if key == "" {
			continue
		}

		s.keys = append(s.keys, sortKey{
			name: key,
			dir:  ternary(direction == "asc", 1, -1),
		})
	}

	return nil
}

func (s *sortableFields[T]) Default(sort string) error {
	if len(s.keys) == 0 {
		return s.parse(sort)
	}

	return nil
}

func (s *sortableFields[T]) Fields(fields map[string]func(a, b T) int) error {
	for i := range s.keys {
		option := s.keys[i].name
		if _, ok := fields[option]; !ok {
			keys := slices.Collect(maps.Keys(fields))
			slices.Sort(keys)
			formatted := strings.Join(keys, ", ")
			return fmt.Errorf("unsupported sort-by option `%s`, must be one or more of [%s] separated by comma", option, formatted)
		}
	}

	s.fields = fields

	return nil
}

func (s *sortableFields[T]) Apply(data []T) {
	slices.SortStableFunc(data, func(a, b T) int {
		for _, key := range s.keys {
			field, ok := s.fields[key.name]
			if !ok {
				continue
			}

			if result := field(a, b) * key.dir; result != 0 {
				return result
			}
		}

		return 0
	})
}
