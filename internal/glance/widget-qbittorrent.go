package glance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	qBittorrentAPIPrefix    = "/api/v2"
	qBittorrentLoginPath    = qBittorrentAPIPrefix + "/auth/login"
	qBittorrentTorrentsPath = qBittorrentAPIPrefix + "/torrents/info"
)

var qbittorrentWidgetTemplate = mustParseTemplate("qbittorrent.html", "widget-base.html")

type qbittorrentWidget struct {
	widgetBase `yaml:",inline"`
	URL        string               `yaml:"url"`
	Username   string               `yaml:"username"`
	Password   string               `yaml:"password"`
	Limit      int                  `yaml:"limit"`
	Torrents   []qbittorrentTorrent `yaml:"-"`
	client     *http.Client         `yaml:"-"`
}

type qbittorrentTorrent struct {
	Name       string  `json:"name"`
	Progress   float64 `json:"progress"`
	State      string  `json:"state"`
	Size       int64   `json:"size"`
	Downloaded int64   `json:"downloaded"`
	Speed      uint64  `json:"dlspeed"`
}

func (widget *qbittorrentWidget) initialize() error {
	widget.
		withTitle("qBittorrent").
		withTitleURL(widget.URL).
		withCacheDuration(time.Second * 5)

	if widget.URL == "" {
		return errors.New("URL is required")
	}

	if _, err := url.Parse(widget.URL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if widget.Limit <= 0 {
		widget.Limit = 5
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("error creating cookie jar: %w", err)
	}

	widget.client = &http.Client{Jar: jar}

	if err := widget.login(); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

func (widget *qbittorrentWidget) login() error {
	loginData := url.Values{}
	loginData.Set("username", widget.Username)
	loginData.Set("password", widget.Password)

	req, err := http.NewRequest("POST", widget.URL+qBittorrentLoginPath, strings.NewReader(loginData.Encode()))
	if err != nil {
		return fmt.Errorf("creating login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", widget.URL)

	resp, err := widget.client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (widget *qbittorrentWidget) update(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, "GET", widget.URL+qBittorrentTorrentsPath, nil)
	if err != nil {
		widget.withError(fmt.Errorf("creating torrents request: %w", err))
		return
	}

	req.Header.Set("Referer", widget.URL)

	resp, err := widget.client.Do(req)
	if err != nil {
		widget.withError(fmt.Errorf("torrents request failed: %w", err))
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		widget.withError(fmt.Errorf("torrents request failed with status %d: %s", resp.StatusCode, string(body)))
		return
	}

	var torrents []qbittorrentTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		widget.withError(fmt.Errorf("decoding torrents response: %w", err))
		return
	}

	sort.Slice(torrents, func(i, j int) bool {
		return torrents[i].Progress > torrents[j].Progress
	})

	if len(torrents) > widget.Limit {
		torrents = torrents[:widget.Limit]
	}

	widget.Torrents = torrents
	widget.withError(nil)
}

func (widget *qbittorrentWidget) Render() template.HTML {
	return widget.renderTemplate(widget, qbittorrentWidgetTemplate)
}
