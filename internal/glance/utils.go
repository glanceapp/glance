package glance

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

var sequentialWhitespacePattern = regexp.MustCompile(`\s+`)
var whitespaceAtBeginningOfLinePattern = regexp.MustCompile(`(?m)^\s+`)

func percentChange(current, previous float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0 // 0% change if both are 0
		}
		return 100 // 100% increase if going from 0 to something
	}

	return (current/previous - 1) * 100
}

func extractDomainFromUrl(u string) string {
	if u == "" {
		return ""
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}

	return strings.TrimPrefix(strings.ToLower(parsed.Host), "www.")
}

func svgPolylineCoordsFromYValues(width float64, height float64, values []float64) string {
	if len(values) < 2 {
		return ""
	}

	verticalPadding := height * 0.02
	height -= verticalPadding * 2
	coordinates := make([]string, len(values))
	distanceBetweenPoints := width / float64(len(values)-1)
	min := slices.Min(values)
	max := slices.Max(values)

	for i := range values {
		coordinates[i] = fmt.Sprintf(
			"%.2f,%.2f",
			float64(i)*distanceBetweenPoints,
			((max-values[i])/(max-min))*height+verticalPadding,
		)
	}

	return strings.Join(coordinates, " ")
}

func maybeCopySliceWithoutZeroValues[T int | float64](values []T) []T {
	if len(values) == 0 {
		return values
	}

	for i := range values {
		if values[i] != 0 {
			continue
		}

		c := make([]T, 0, len(values)-1)

		for i := range values {
			if values[i] != 0 {
				c = append(c, values[i])
			}
		}

		return c
	}

	return values
}

var urlSchemePattern = regexp.MustCompile(`^[a-z]+:\/\/`)

func stripURLScheme(url string) string {
	return urlSchemePattern.ReplaceAllString(url, "")
}

func isRunningInsideDockerContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func prefixStringLines(prefix string, s string) string {
	lines := strings.Split(s, "\n")

	for i, line := range lines {
		lines[i] = prefix + line
	}

	return strings.Join(lines, "\n")
}

func limitStringLength(s string, max int) (string, bool) {
	asRunes := []rune(s)

	if len(asRunes) > max {
		return string(asRunes[:max]), true
	}

	return s, false
}

func parseRFC3339Time(t string) time.Time {
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return time.Now()
	}

	return parsed
}

func normalizeVersionFormat(version string) string {
	version = strings.ToLower(strings.TrimSpace(version))

	if len(version) > 0 && version[0] != 'v' {
		return "v" + version
	}

	return version
}

func titleToSlug(s string) string {
	s = strings.ToLower(s)
	s = sequentialWhitespacePattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	return s
}

func fileServerWithCache(fs http.FileSystem, cacheDuration time.Duration) http.Handler {
	server := http.FileServer(fs)
	cacheControlValue := fmt.Sprintf("public, max-age=%d", int(cacheDuration.Seconds()))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: fix always setting cache control even if the file doesn't exist
		w.Header().Set("Cache-Control", cacheControlValue)
		server.ServeHTTP(w, r)
	})
}

func executeTemplateToString(t *template.Template, data any) (string, error) {
	var b bytes.Buffer
	err := t.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return b.String(), nil
}

func stringToBool(s string) bool {
	return s == "true" || s == "yes"
}

func itemAtIndexOrDefault[T any](items []T, index int, def T) T {
	if index >= len(items) {
		return def
	}

	return items[index]
}

func ternary[T any](condition bool, a, b T) T {
	if condition {
		return a
	}

	return b
}

// Having compile time errors about unused variables is cool and all, but I don't want to
// have to constantly comment out my code while I'm working on it and testing things out
func ItsUsedTrustMeBro(...any) {}

func hslToHex(h, s, l float64) string {
	s /= 100.0
	l /= 100.0

	var r, g, b float64

	if s == 0 {
		r, g, b = l, l, l
	} else {
		hueToRgb := func(p, q, t float64) float64 {
			if t < 0 {
				t += 1
			}
			if t > 1 {
				t -= 1
			}
			if t < 1.0/6.0 {
				return p + (q-p)*6.0*t
			}
			if t < 1.0/2.0 {
				return q
			}
			if t < 2.0/3.0 {
				return p + (q-p)*(2.0/3.0-t)*6.0
			}
			return p
		}

		q := 0.0
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}

		p := 2*l - q

		h /= 360.0

		r = hueToRgb(p, q, h+1.0/3.0)
		g = hueToRgb(p, q, h)
		b = hueToRgb(p, q, h-1.0/3.0)
	}

	ir := int(math.Round(r * 255.0))
	ig := int(math.Round(g * 255.0))
	ib := int(math.Round(b * 255.0))

	ir = int(math.Max(0, math.Min(255, float64(ir))))
	ig = int(math.Max(0, math.Min(255, float64(ig))))
	ib = int(math.Max(0, math.Min(255, float64(ib))))

	return fmt.Sprintf("#%02x%02x%02x", ir, ig, ib)
}
