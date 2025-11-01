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

var durationFieldPattern = regexp.MustCompile(`^(\d+)(s|m|h|d|w|mo|y)$`)

type durationField time.Duration

func parseDurationValue(value string) (time.Duration, error) {
	matches := durationFieldPattern.FindStringSubmatch(value)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid format for value `%s`, must be a number followed by one of: s, m, h, d, w, mo, y", value)
	}

	duration, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}

	switch matches[2] {
	case "s":
		return time.Duration(duration) * time.Second, nil
	case "m":
		return time.Duration(duration) * time.Minute, nil
	case "h":
		return time.Duration(duration) * time.Hour, nil
	case "d":
		return time.Duration(duration) * 24 * time.Hour, nil
	case "w":
		return time.Duration(duration) * 7 * 24 * time.Hour, nil
	case "mo":
		return time.Duration(duration) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(duration) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", matches[2])
	}
}

func (d *durationField) UnmarshalYAML(node *yaml.Node) error {
	var value string

	if err := node.Decode(&value); err != nil {
		return err
	}

	parsedDuration, err := parseDurationValue(value)
	if err != nil {
		return fmt.Errorf("line %d: %w", node.Line, err)
	}

	*d = durationField(parsedDuration)

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

	if ext != "svg" && ext != "png" && ext != "webp" {
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

type filterableData interface {
	filterableField(field string) any
}

type filterableFields[T filterableData] struct {
	filters       []func(T) bool
	FilteredCount int  `yaml:"-"`
	AllFiltered   bool `yaml:"-"`
}

func (f *filterableFields[T]) Apply(items []T) []T {
	if len(f.filters) == 0 {
		f.FilteredCount = 0
		f.AllFiltered = false
		return items
	}

	filtered := make([]T, 0, len(items))

	for _, item := range items {
		include := true
		for _, shouldInclude := range f.filters {
			if !shouldInclude(item) {
				include = false
				break
			}
		}
		if include {
			filtered = append(filtered, item)
		}
	}

	f.FilteredCount = len(items) - len(filtered)
	f.AllFiltered = f.FilteredCount == len(items)

	return filtered
}

func (f *filterableFields[T]) UnmarshalYAML(node *yaml.Node) error {
	untypedFilters := make(map[string]any)
	if err := node.Decode(&untypedFilters); err != nil {
		return errors.New("filters must be defined as an object where each key is the name of a field")
	}

	rawFilters := make(map[string][]string)
	for key, value := range untypedFilters {
		rawFilters[key] = []string{}

		switch vt := value.(type) {
		case string:
			rawFilters[key] = append(rawFilters[key], vt)
		case []any:
			for _, item := range vt {
				if str, ok := item.(string); ok {
					rawFilters[key] = append(rawFilters[key], str)
				} else {
					return fmt.Errorf("filter value in array for %s must be a string, got %T", key, item)
				}
			}
		case nil:
			continue // skip empty filters
		default:
			return fmt.Errorf("filter value for %s must be a string or an array, got %T", key, value)
		}
	}

	makeStringFilter := func(key string, values []string) (func(T) bool, error) {
		parsedFilters := []func(string) bool{}

		for _, value := range values {
			value, negative := strings.CutPrefix(value, "!")

			if value == "" {
				return nil, errors.New("value is empty")
			}

			if pattern, ok := strings.CutPrefix(value, "re:"); ok {
				re, err := regexp.Compile(pattern)
				if err != nil {
					return nil, fmt.Errorf("value `%s`: %w", value, err)
				}

				parsedFilters = append(parsedFilters, func(s string) bool {
					return negative != re.MatchString(s)
				})

				continue
			}

			value = strings.ToLower(value)
			parsedFilters = append(parsedFilters, func(s string) bool {
				return negative != strings.Contains(strings.ToLower(s), value)
			})
		}

		return func(item T) bool {
			value, ok := item.filterableField(key).(string)
			if !ok {
				return false
			}

			for i := range parsedFilters {
				if !parsedFilters[i](value) {
					return false
				}
			}

			return true
		}, nil
	}

	makeIntFilter := func(key string, values []string) (func(T) bool, error) {
		parsedFilters := []func(int) bool{}

		parseNumber := func(value string) (int, error) {
			var multiplier int
			if strings.HasSuffix(value, "k") {
				multiplier = 1_000
				value = strings.TrimSuffix(value, "k")
			} else if strings.HasSuffix(value, "m") {
				multiplier = 1_000_000
				value = strings.TrimSuffix(value, "m")
			} else {
				multiplier = 1
			}

			num, err := strconv.Atoi(value)
			if err != nil {
				return 0, fmt.Errorf("invalid number format for key %s: %w", key, err)
			}

			return num * multiplier, nil
		}

		for _, value := range values {
			if number, ok := strings.CutPrefix(value, "<"); ok {
				num, err := parseNumber(number)
				if err != nil {
					return nil, err
				}

				parsedFilters = append(parsedFilters, func(v int) bool {
					return v < num
				})
			} else if number, ok := strings.CutPrefix(value, ">"); ok {
				num, err := parseNumber(number)
				if err != nil {
					return nil, err
				}

				parsedFilters = append(parsedFilters, func(v int) bool {
					return v > num
				})
			} else {
				num, err := parseNumber(value)
				if err != nil {
					return nil, err
				}

				parsedFilters = append(parsedFilters, func(v int) bool {
					return v == num
				})
			}
		}

		return func(item T) bool {
			value, ok := item.filterableField(key).(int)
			if !ok {
				return false
			}

			for i := range parsedFilters {
				if !parsedFilters[i](value) {
					return false
				}
			}

			return true
		}, nil
	}

	makeTimeFilter := func(key string, values []string) (func(T) bool, error) {
		parsedFilters := []func(time.Time) bool{}

		for _, value := range values {
			if number, ok := strings.CutPrefix(value, "<"); ok {
				duration, err := parseDurationValue(number)
				if err != nil {
					return nil, err
				}

				parsedFilters = append(parsedFilters, func(t time.Time) bool {
					return time.Since(t) < duration
				})
			} else if number, ok := strings.CutPrefix(value, ">"); ok {
				duration, err := parseDurationValue(number)
				if err != nil {
					return nil, err
				}

				parsedFilters = append(parsedFilters, func(t time.Time) bool {
					return time.Since(t) > duration
				})
			} else {
				return nil, fmt.Errorf("invalid time filter format for value `%s`", value)
			}
		}

		return func(item T) bool {
			value, ok := item.filterableField(key).(time.Time)
			if !ok {
				return false
			}

			for i := range parsedFilters {
				if !parsedFilters[i](value) {
					return false
				}
			}

			return true
		}, nil
	}

	var data T
	for key, values := range rawFilters {
		if len(values) == 0 {
			continue
		}

		value := data.filterableField(key)

		if value == nil {
			return fmt.Errorf("filter with key `%s` is not supported", key)
		}

		var filter func(T) bool
		var err error

		switch v := value.(type) {
		case string:
			filter, err = makeStringFilter(key, values)
		case int:
			filter, err = makeIntFilter(key, values)
		case time.Time:
			filter, err = makeTimeFilter(key, values)
		default:
			return fmt.Errorf("unsupported filter type for key %s: %T", key, v)
		}

		if err != nil {
			return fmt.Errorf("failed to create filter for key %s: %w", key, err)
		}

		f.filters = append(f.filters, filter)
	}

	return nil
}
