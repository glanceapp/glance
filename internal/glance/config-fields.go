package glance

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var hslColorFieldPattern = regexp.MustCompile(`^(?:hsla?\()?([\d\.]+)(?: |,)+([\d\.]+)%?(?: |,)+([\d\.]+)%?\)?$`)

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

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := hslColorFieldPattern.FindStringSubmatch(value)

	if len(matches) != 4 {
		return fmt.Errorf("invalid HSL color format: %s", value)
	}

	hue, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return err
	}

	if hue > hslHueMax {
		return fmt.Errorf("HSL hue must be between 0 and %d", hslHueMax)
	}

	saturation, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return err
	}

	if saturation > hslSaturationMax {
		return fmt.Errorf("HSL saturation must be between 0 and %d", hslSaturationMax)
	}

	lightness, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return err
	}

	if lightness > hslLightnessMax {
		return fmt.Errorf("HSL lightness must be between 0 and %d", hslLightnessMax)
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

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := durationFieldPattern.FindStringSubmatch(value)

	if len(matches) != 3 {
		return fmt.Errorf("invalid duration format: %s", value)
	}

	duration, err := strconv.Atoi(matches[1])
	if err != nil {
		return err
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
	AutoInvert bool
}

func newCustomIconField(value string) customIconField {
	const autoInvertPrefix = "auto-invert "
	field := customIconField{}

	if strings.HasPrefix(value, autoInvertPrefix) {
		field.AutoInvert = true
		value = strings.TrimPrefix(value, autoInvertPrefix)
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
					return fmt.Errorf("invalid query parameter value type: %T", item)
				}
			}
		default:
			return fmt.Errorf("invalid query parameter value type: %T", value)
		}
	}

	return nil
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
