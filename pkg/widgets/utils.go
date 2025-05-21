package widgets

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
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
