package glance

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var hslColorFieldPattern = regexp.MustCompile(`^(?:hsla?\()?(\d{1,3})(?: |,)+(\d{1,3})%?(?: |,)+(\d{1,3})%?\)?$`)

const (
	hslHueMax        = 360
	hslSaturationMax = 100
	hslLightnessMax  = 100
)

type hslColorField struct {
	Hue        uint16
	Saturation uint8
	Lightness  uint8
}

func (c *hslColorField) String() string {
	return fmt.Sprintf("hsl(%d, %d%%, %d%%)", c.Hue, c.Saturation, c.Lightness)
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

	hue, err := strconv.ParseUint(matches[1], 10, 16)
	if err != nil {
		return err
	}

	if hue > hslHueMax {
		return fmt.Errorf("HSL hue must be between 0 and %d", hslHueMax)
	}

	saturation, err := strconv.ParseUint(matches[2], 10, 8)
	if err != nil {
		return err
	}

	if saturation > hslSaturationMax {
		return fmt.Errorf("HSL saturation must be between 0 and %d", hslSaturationMax)
	}

	lightness, err := strconv.ParseUint(matches[3], 10, 8)
	if err != nil {
		return err
	}

	if lightness > hslLightnessMax {
		return fmt.Errorf("HSL lightness must be between 0 and %d", hslLightnessMax)
	}

	c.Hue = uint16(hue)
	c.Saturation = uint8(saturation)
	c.Lightness = uint8(lightness)

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
	URL        string
	IsFlatIcon bool
	// TODO: along with whether the icon is flat, we also need to know
	// whether the icon is black or white by default in order to properly
	// invert the color based on the theme being light or dark
}

func newCustomIconField(value string) customIconField {
	field := customIconField{}

	prefix, icon, found := strings.Cut(value, ":")
	if !found {
		field.URL = value
		return field
	}

	switch prefix {
	case "si":
		field.URL = "https://cdn.jsdelivr.net/npm/simple-icons@latest/icons/" + icon + ".svg"
		field.IsFlatIcon = true
	case "di", "sh":
		// syntax: di:<icon_name>[.svg|.png]
		// syntax: sh:<icon_name>[.svg|.png]
		// if the icon name is specified without extension, it is assumed to be wanting the SVG icon
		// otherwise, specify the extension of either .svg or .png to use either of the CDN offerings
		// any other extension will be interpreted as .svg
		basename, ext, found := strings.Cut(icon, ".")
		if !found {
			ext = "svg"
			basename = icon
		}

		if ext != "svg" && ext != "png" {
			ext = "svg"
		}

		if prefix == "di" {
			field.URL = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/" + ext + "/" + basename + "." + ext
		} else {
			field.URL = "https://cdn.jsdelivr.net/gh/selfhst/icons/" + ext + "/" + basename + "." + ext
		}
	default:
		field.URL = value
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
