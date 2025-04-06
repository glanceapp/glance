package glance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
)

var torrentWidgetTemplate = mustParseTemplate("torrents.html", "widget-base.html")

const (
	delugeClient = "deluge"
)

type TorrentClient interface {
	GetTorrents() ([]Torrent, error)
}

type Torrent struct {
	Name        string  `json:"name"`
	Progress    float64 `json:"progress"`
	State       string  `json:"state"`
	Size        uint64  `json:"size"`
	Downloaded  uint64  `json:"downloaded"`
	Speed       uint64  `json:"dlspeed"`
	CompletedOn uint64  `json:"completed_on"`
}

type torrentWidget struct {
	widgetBase `yaml:",inline"`
	URL        string        `yaml:"url"`
	Username   string        `yaml:"username"`
	Password   string        `yaml:"password"`
	Limit      int           `yaml:"limit"`
	ClientType string        `yaml:"client"`
	Torrents   []Torrent     `yaml:"-"`
	client     TorrentClient `yaml:"-"`
}

func (widget *torrentWidget) initialize() error {
	widget.
		withTitle("Torrents").
		withTitleURL(widget.URL).
		withCacheDuration(-1)

	_, err := url.Parse(widget.URL)
	if err != nil {
		return fmt.Errorf("parsing URL: %v", err)
	}

	var client TorrentClient
	switch widget.ClientType {
	case delugeClient:
		url := strings.TrimRight(widget.URL, "/") + "/json"
		client, err = createDelugeClient(url, widget.Password)
	default:
		return errors.New("unsupported torrent client type")
	}

	if err != nil {
		return err
	}

	widget.client = client
	return nil
}

func (widget *torrentWidget) update(ctx context.Context) {
	torrents, err := widget.client.GetTorrents()
	if err != nil {
		widget.withError(err)
		return
	}
	// Sort torrents by progress
	sort.Slice(torrents, func(i, j int) bool {
		if torrents[i].Progress == 100 && torrents[j].Progress == 100 {
			return torrents[i].CompletedOn > torrents[j].CompletedOn
		}
		return torrents[i].Progress < torrents[j].Progress
	})

	// Apply limit if necessary
	if len(torrents) > widget.Limit {
		torrents = torrents[:widget.Limit]
	}

	// Store the sorted torrents in the widget
	widget.Torrents = torrents
	widget.withError(nil).scheduleNextUpdate()
}

type DelugeClient struct {
	client *http.Client
	url    string
	id     uint64
}

type JSONRPCRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
	ID     uint64        `json:"id"`
}

type JSONRPCError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type JSONRPCResponse[T interface{}] struct {
	Result T             `json:"result"`
	Error  *JSONRPCError `json:"error"`
	ID     uint64        `json:"id"`
}

type DelugeTorrent struct {
	Name            string  `json:"name"`
	PercentProgress float64 `json:"progress"`
	Status          string  `json:"state"`
	Size            uint64  `json:"total_size"`
	Downloaded      uint64  `json:"all_time_download"`
	DownloadSpeed   uint64  `json:"download_payload_rate"`
	CompletedOn     uint64  `json:"completed_time"`
}

func createDelugeClient(url, password string) (*DelugeClient, error) {
	jar, _ := cookiejar.New(nil)
	http := &http.Client{
		Jar: jar,
	}

	client := DelugeClient{http, url, 0}

	res, err := rpcRequest[bool](&client, "auth.login", []interface{}{password})
	if err != nil {
		return nil, err
	}

	if !res.Result {
		return nil, errors.New("not authenticated")
	}

	return &client, nil
}

func rpcRequest[T interface{}](deluge *DelugeClient, method string, params []interface{}) (*JSONRPCResponse[T], error) {
	bodyBytes, _ := json.Marshal(JSONRPCRequest{Method: method, Params: params, ID: deluge.id})

	req, _ := http.NewRequest("POST", deluge.url, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := deluge.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var response JSONRPCResponse[T]
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, errors.New(response.Error.Message)
	}

	return &response, nil
}

func (deluge *DelugeClient) GetTorrents() ([]Torrent, error) {
	filterDict := map[string]interface{}{}
	keys := []string{"name", "state", "progress", "total_size", "all_time_download", "download_payload_rate", "completed_time"}
	res, err := rpcRequest[map[string]DelugeTorrent](deluge, "core.get_torrents_status", []interface{}{filterDict, keys})
	if err != nil {
		return nil, err
	}
	var torrents []Torrent
	for _, torrent := range res.Result {
		torrents = append(torrents, Torrent{
			Name:        torrent.Name,
			Progress:    math.Round(torrent.PercentProgress*100) / 100,
			State:       torrent.Status,
			Size:        torrent.Size,
			Downloaded:  torrent.Downloaded,
			Speed:       torrent.DownloadSpeed,
			CompletedOn: torrent.CompletedOn,
		})
	}
	return torrents, nil
}

func (widget *torrentWidget) Render() template.HTML {
	return widget.renderTemplate(widget, torrentWidgetTemplate)
}
