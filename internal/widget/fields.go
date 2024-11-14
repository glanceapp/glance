package widget

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

var HSLColorPattern = regexp.MustCompile(`^(?:hsla?\()?(\d{1,3})(?: |,)+(\d{1,3})%?(?: |,)+(\d{1,3})%?\)?$`)
var EnvFieldPattern = regexp.MustCompile(`^\${([A-Z_]+)}$`)

const (
	HSLHueMax        = 360
	HSLSaturationMax = 100
	HSLLightnessMax  = 100
)

type HSLColorField struct {
	Hue        uint16
	Saturation uint8
	Lightness  uint8
}

type IconSource uint8

const (
	IconURI IconSource = iota
	SimpleIcon
	DashboardIcon
)

func (c *HSLColorField) String() string {
	return fmt.Sprintf("hsl(%d, %d%%, %d%%)", c.Hue, c.Saturation, c.Lightness)
}

func (c *HSLColorField) AsCSSValue() template.CSS {
	return template.CSS(c.String())
}

func (c *HSLColorField) UnmarshalYAML(node *yaml.Node) error {
	var value string

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := HSLColorPattern.FindStringSubmatch(value)

	if len(matches) != 4 {
		return fmt.Errorf("invalid HSL color format: %s", value)
	}

	hue, err := strconv.ParseUint(matches[1], 10, 16)

	if err != nil {
		return err
	}

	if hue > HSLHueMax {
		return fmt.Errorf("HSL hue must be between 0 and %d", HSLHueMax)
	}

	saturation, err := strconv.ParseUint(matches[2], 10, 8)

	if err != nil {
		return err
	}

	if saturation > HSLSaturationMax {
		return fmt.Errorf("HSL saturation must be between 0 and %d", HSLSaturationMax)
	}

	lightness, err := strconv.ParseUint(matches[3], 10, 8)

	if err != nil {
		return err
	}

	if lightness > HSLLightnessMax {
		return fmt.Errorf("HSL lightness must be between 0 and %d", HSLLightnessMax)
	}

	c.Hue = uint16(hue)
	c.Saturation = uint8(saturation)
	c.Lightness = uint8(lightness)

	return nil
}

var DurationPattern = regexp.MustCompile(`^(\d+)(s|m|h|d)$`)

type DurationField time.Duration

func (d *DurationField) UnmarshalYAML(node *yaml.Node) error {
	var value string

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := DurationPattern.FindStringSubmatch(value)

	if len(matches) != 3 {
		return fmt.Errorf("invalid duration format: %s", value)
	}

	duration, err := strconv.Atoi(matches[1])

	if err != nil {
		return err
	}

	switch matches[2] {
	case "s":
		*d = DurationField(time.Duration(duration) * time.Second)
	case "m":
		*d = DurationField(time.Duration(duration) * time.Minute)
	case "h":
		*d = DurationField(time.Duration(duration) * time.Hour)
	case "d":
		*d = DurationField(time.Duration(duration) * 24 * time.Hour)
	}

	return nil
}

type OptionalEnvString string

func (f *OptionalEnvString) UnmarshalYAML(node *yaml.Node) error {
	var value string

	err := node.Decode(&value)

	if err != nil {
		return err
	}

	matches := EnvFieldPattern.FindStringSubmatch(value)

	if len(matches) != 2 {
		*f = OptionalEnvString(value)

		return nil
	}

	value, found := os.LookupEnv(matches[1])

	if !found {
		return fmt.Errorf("environment variable %s not found", matches[1])
	}

	*f = OptionalEnvString(value)

	return nil
}

func (f *OptionalEnvString) String() string {
	return string(*f)
}

func toIconURIIfPrefixed(icon string) (string, IconSource) {
	var prefix, iconstr string

	prefix, iconstr, found := strings.Cut(icon, ":")

	if !found {
		return icon, IconURI
	}

	// syntax: si:<icon_name>
	if prefix == "si" {
		icon = "https://cdnjs.cloudflare.com/ajax/libs/simple-icons/11.14.0/" + iconstr + ".svg"
		return icon, SimpleIcon
	}

	// syntax: di:<icon_name>[.svg|.png]
	if prefix == "di" {
		// if the icon name is specified without extension, it is assumed to be wanting the SVG icon
		// otherwise, specify the extension of either .svg or .png to use either of the CDN offerings
		// any other extension will be interpreted as .svg
		var basename, ext string

		basename, ext, found := strings.Cut(iconstr, ".")

		if !found {
			ext = "svg"
			basename = iconstr
		}

		icon = "https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons@master/" + ext + "/" + basename + "." + ext
		return icon, DashboardIcon
	}

	return icon, IconURI
}
