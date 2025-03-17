package glance

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"
)

var tautulliTemplate = mustParseTemplate("tautulli.html", "widget-base.html")

type tautulliWidget struct {
	widgetBase  `yaml:",inline"`
	ApiKey      string `yaml:"apikey"`
	TautulliUrl string `yaml:"tautulli-url"`
	ProgressBar bool   `yaml:"progress-bar"`
	Users       Users  `yaml:"-"` // Add this to store the fetched data
}

func (widget *tautulliWidget) initialize() error {
	widget.
		withTitle("Tautulli").
		withTitleURL(widget.TautulliUrl).
		withCacheDuration(1 * time.Minute)

	return nil
}

func (widget *tautulliWidget) update(ctx context.Context) {
	users, err := fetchTautulliSessions(widget.ApiKey, widget.TautulliUrl)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Users = users
}

func (widget *tautulliWidget) Render() template.HTML {
	return widget.renderTemplate(widget, tautulliTemplate)
}

type tautulliResponseJson struct {
	Response struct {
		Data struct {
			StreamCount string `json:"stream_count"`
			Sessions    []struct {
				User             string `json:"user"`
				MediaType        string `json:"media_type"`
				LibraryName      string `json:"library_name"`
				GrandparentTitle string `json:"grandparent_title"`
				ParentTitle      string `json:"parent_title"`
				Title            string `json:"title"`
				ParentMediaIndex string `json:"parent_media_index"`
				MediaIndex       string `json:"media_index"`
				ProgressPercent  string `json:"progress_percent"`
			} `json:"sessions"`
		} `json:"data"`
	} `json:"response"`
}

type Session struct {
	LibraryName      string
	MediaType        string
	GrandparentTitle string
	ParentTitle      string
	Title            string
	ParentMediaIndex string
	MediaIndex       string
	ProgressPercent  string
}

type User struct {
	Name     string
	Sessions []Session
}

type Users []User

func fetchTautulliSessions(apiKey string, tautulliUrl string) (Users, error) {
	url := fmt.Sprintf("%s/api/v2?apikey=%s&cmd=get_activity", tautulliUrl, apiKey)

	request, _ := http.NewRequest("GET", url, nil) // Fixed 'mil' to 'nil'
	response, err := decodeJsonFromRequest[tautulliResponseJson](defaultHTTPClient, request)
	if err != nil {
		return nil, fmt.Errorf("%w: could not fetch plex's sessions", errNoContent)
	}

	// Map by username
	userMap := make(map[string]User)

	for _, session := range response.Response.Data.Sessions {
		sessionData := Session{
			LibraryName:      session.LibraryName,
			MediaType:        session.MediaType,
			GrandparentTitle: session.GrandparentTitle,
			ParentTitle:      session.ParentTitle,
			Title:            session.Title,
			ParentMediaIndex: session.ParentMediaIndex,
			MediaIndex:       session.MediaIndex,
			ProgressPercent:  session.ProgressPercent,
		}

		// If user exists, append the session
		// Otherwise, create new entry
		if user, exists := userMap[session.User]; exists {
			user.Sessions = append(user.Sessions, sessionData)
			userMap[session.User] = user
		} else {
			userMap[session.User] = User{
				Name:     session.User,
				Sessions: []Session{sessionData},
			}
		}
	}

	// Convert map to slice of Users
	users := make(Users, 0, len(userMap))
	for _, user := range userMap {
		users = append(users, user)
	}

	return users, nil
}
