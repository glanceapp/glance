package glance

import (
	"fmt"
	"html/template"
	"os"
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

func (c *hslColorField) AsCSSValue() template.CSS {
	return template.CSS(c.String())
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

var optionalEnvFieldPattern = regexp.MustCompile(`(^|.)\$\{([A-Z0-9_]+)\}`)

type optionalEnvField string

func (f *optionalEnvField) UnmarshalYAML(node *yaml.Node) error {
	var value string

	err := node.Decode(&value)
	if err != nil {
		return err
	}

	replaced := optionalEnvFieldPattern.ReplaceAllStringFunc(value, func(match string) string {
		if err != nil {
			return ""
		}

		groups := optionalEnvFieldPattern.FindStringSubmatch(match)

		if len(groups) != 3 {
			return match
		}

		prefix, key := groups[1], groups[2]

		if prefix == `\` {
			if len(match) >= 2 {
				return match[1:]
			} else {
				return ""
			}
		}

		value, found := os.LookupEnv(key)
		if !found {
			err = fmt.Errorf("environment variable %s not found", key)
			return ""
		}

		return prefix + value
	})

	if err != nil {
		return err
	}

	*f = optionalEnvField(replaced)

	return nil
}

func (f *optionalEnvField) String() string {
	return string(*f)
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
			field.URL = "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons@master/" + ext + "/" + basename + "." + ext
		} else {
			field.URL = "https://cdn.jsdelivr.net/gh/selfhst/icons@main/" + ext + "/" + basename + "." + ext
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
