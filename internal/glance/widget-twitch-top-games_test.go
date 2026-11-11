package glance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// fakeTwitchDirectories behaves like Twitch's GQL endpoint: it returns the requested
// number of categories and rejects limits above 100.
func fakeTwitchDirectories(t *testing.T) roundTripperFunc {
	return func(r *http.Request) (*http.Response, error) {
		var body []struct {
			Variables struct {
				Limit int `json:"limit"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("could not decode request body: %v", err)
		}

		limit := body[0].Variables.Limit
		var response string
		if limit < 1 || limit > 100 {
			response = `[{"errors":[{"message":"argument 'first' value must be between 1 and 100.","path":["directoriesWithTags"]}],"data":{"directoriesWithTags":null}}]`
		} else {
			edges := make([]string, 0, limit)
			for i := range limit {
				edges = append(edges, fmt.Sprintf(`{"node":{"slug":"game-%d","name":"Game %d"}}`, i, i))
			}
			response = `[{"data":{"directoriesWithTags":{"edges":[` + strings.Join(edges, ",") + `]}}}]`
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(response)),
			Request:    r,
		}, nil
	}
}

func TestFetchTopGamesFromTwitchWithManyExcludes(t *testing.T) {
	transport := defaultHTTPClient.Transport
	defaultHTTPClient.Transport = fakeTwitchDirectories(t)
	t.Cleanup(func() { defaultHTTPClient.Transport = transport })

	exclude := make([]string, 0, 95)
	for i := range 95 {
		exclude = append(exclude, fmt.Sprintf("game-%d", i))
	}

	categories, err := fetchTopGamesFromTwitch(exclude, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(categories) != 5 {
		t.Fatalf("got %d categories, want 5", len(categories))
	}

	if categories[0].Slug != "game-95" {
		t.Errorf("first category = %q, want %q", categories[0].Slug, "game-95")
	}
}
