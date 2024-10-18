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
var EnvFieldPattern = regexp.MustCompile(`(^|.)\$\{([A-Z_]+)\}`)

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

	replaced := EnvFieldPattern.ReplaceAllStringFunc(value, func(whole string) string {
		if err != nil {
			return ""
		}

		groups := EnvFieldPattern.FindStringSubmatch(whole)

		if len(groups) != 3 {
			return whole
		}

		prefix, key := groups[1], groups[2]

		if prefix == `\` {
			if len(whole) >= 2 {
				return whole[1:]
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

	*f = OptionalEnvString(replaced)

	return nil
}

func (f *OptionalEnvString) String() string {
	return string(*f)
}

func toSimpleIconIfPrefixed(icon string) (string, bool) {
	if !strings.HasPrefix(icon, "si:") {
		return icon, false
	}

	icon = strings.TrimPrefix(icon, "si:")
	icon = "https://cdn.jsdelivr.net/npm/simple-icons@latest/icons/" + icon + ".svg"

	return icon, true
}
