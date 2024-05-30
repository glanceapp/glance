package feed

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var (
	ErrNoContent      = errors.New("failed to retrieve any content")
	ErrPartialContent = errors.New("failed to retrieve some of the content")
)

func percentChange(current, previous float64) float64 {
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

func SvgPolylineCoordsFromYValues(width float64, height float64, values []float64) string {
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

func limitStringLength(s string, max int) (string, bool) {
	asRunes := []rune(s)

	if len(asRunes) > max {
		return string(asRunes[:max]), true
	}

	return s, false
}
