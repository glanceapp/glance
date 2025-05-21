package sources

import (
	"errors"
	"fmt"
	"math"
	"time"
)

func NewSource(widgetType string) (Source, error) {
	if widgetType == "" {
		return nil, errors.New("widget 'type' property is empty or not specified")
	}

	var s Source

	switch widgetType {
	case "mastodon":
		s = &mastodonSource{}
	case "hacker-news":
		s = &hackerNewsSource{}
	case "reddit":
		s = &redditSource{}
	case "lobsters":
		s = &lobstersSource{}
	case "rss":
		s = &rssSource{}
	case "releases":
		s = &githubReleasesSource{}
	case "issues":
		s = &githubIssuesSource{}
	case "change-detection":
		s = &changeDetectionWidget{}
	default:
		return nil, fmt.Errorf("unknown widget type: %s", widgetType)
	}

	return s, nil
}

// Source TODO(pulse): Feed() returns cached activities, but refactor it to fetch fresh activities given filters and cache them in a global activity registry.
type Source interface {
	// Feed return cached feed entries in a standard Activity format.
	Feed() []Activity
	RequiresUpdate(now *time.Time) bool
}

type Activity interface {
	UID() string
	Title() string
	Body() string
	URL() string
	ImageURL() string
	CreatedAt() time.Time
	// TODO: Add Metadata() that returns custom fields?
}

type cacheType int

const (
	cacheTypeInfinite cacheType = iota
	cacheTypeDuration
	cacheTypeOnTheHour
)

type sourceBase struct {
	ID                  uint64           `yaml:"-"`
	Title               string           `yaml:"title"`
	TitleURL            string           `yaml:"title-url"`
	ContentAvailable    bool             `yaml:"-"`
	Error               error            `yaml:"-"`
	Notice              error            `yaml:"-"`
	Providers           *sourceProviders `yaml:"-"`
	CustomCacheDuration durationField    `yaml:"cache"`
	cacheDuration       time.Duration    `yaml:"-"`
	cacheType           cacheType        `yaml:"-"`
	nextUpdate          time.Time        `yaml:"-"`
	updateRetriedTimes  int              `yaml:"-"`
}

// TODO(pulse): Do we need this?
type sourceProviders struct {
	assetResolver func(string) string
}

func (w *sourceBase) withTitle(title string) *sourceBase {
	if w.Title == "" {
		w.Title = title
	}

	return w
}

func (w *sourceBase) withTitleURL(titleURL string) *sourceBase {
	if w.TitleURL == "" {
		w.TitleURL = titleURL
	}

	return w
}

func (w *sourceBase) RequiresUpdate(now *time.Time) bool {
	if w.cacheType == cacheTypeInfinite {
		return false
	}

	if w.nextUpdate.IsZero() {
		return true
	}

	return now.After(w.nextUpdate)
}

func (w *sourceBase) withCacheDuration(duration time.Duration) *sourceBase {
	w.cacheType = cacheTypeDuration

	if duration == -1 || w.CustomCacheDuration == 0 {
		w.cacheDuration = duration
	} else {
		w.cacheDuration = time.Duration(w.CustomCacheDuration)
	}

	return w
}

func (w *sourceBase) withCacheOnTheHour() *sourceBase {
	w.cacheType = cacheTypeOnTheHour

	return w
}

func (w *sourceBase) getNextUpdateTime() time.Time {
	now := time.Now()

	if w.cacheType == cacheTypeDuration {
		return now.Add(w.cacheDuration)
	}

	if w.cacheType == cacheTypeOnTheHour {
		return now.Add(time.Duration(
			((60-now.Minute())*60)-now.Second(),
		) * time.Second)
	}

	return time.Time{}
}

func (w *sourceBase) scheduleNextUpdate() *sourceBase {
	w.nextUpdate = w.getNextUpdateTime()
	w.updateRetriedTimes = 0

	return w
}

func (w *sourceBase) scheduleEarlyUpdate() *sourceBase {
	w.updateRetriedTimes++

	if w.updateRetriedTimes > 5 {
		w.updateRetriedTimes = 5
	}

	nextEarlyUpdate := time.Now().Add(time.Duration(math.Pow(float64(w.updateRetriedTimes), 2)) * time.Minute)
	nextUsualUpdate := w.getNextUpdateTime()

	if nextEarlyUpdate.After(nextUsualUpdate) {
		w.nextUpdate = nextUsualUpdate
	} else {
		w.nextUpdate = nextEarlyUpdate
	}

	return w
}

func (w *sourceBase) withNotice(err error) *sourceBase {
	w.Notice = err

	return w
}

func (w *sourceBase) withError(err error) *sourceBase {
	if err == nil && !w.ContentAvailable {
		w.ContentAvailable = true
	}

	w.Error = err

	return w
}

func (s *sourceBase) canContinueUpdateAfterHandlingErr(err error) bool {
	// TODO: needs covering more edge cases.
	// if there's partial content and we update early there's a chance
	// the early update returns even less content than the initial update.
	// need some kind of mechanism that tells us whether we should update early
	// or not depending on the number of things that failed during the initial
	// and subsequent update and how they failed - ie whether it was server
	// error (like gateway timeout, do retry early) or client error (like
	// hitting a rate limit, don't retry early). will require reworking a
	// good amount of code in the feed package and probably having a custom
	// error type that holds more information because screw wrapping errors.
	// alternatively have a resource cache and only refetch the failed resources,
	// then rebuild the widget.

	if err != nil {
		s.scheduleEarlyUpdate()

		if !errors.Is(err, errPartialContent) {
			s.withError(err)
			s.withNotice(nil)
			return false
		}

		s.withError(nil)
		s.withNotice(err)
		return true
	}

	s.withNotice(nil)
	s.withError(nil)
	s.scheduleNextUpdate()
	return true
}
