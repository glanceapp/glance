package widgets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources"
	"html/template"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

var widgetIDCounter atomic.Uint64

func newWidget(widgetType string) (widget, error) {
	if widgetType == "" {
		return nil, errors.New("widget 'type' property is empty or not specified")
	}

	var w widget

	switch widgetType {
	case "group":
		w = &groupWidget{}
	case "split-column":
		w = &splitColumnWidget{}
	default:
		// widget type is treated as a data source type in this case,
		// which depends on the base widget that renders the generic widget display card
		w = &widgetBase{}
	}

	w.setID(widgetIDCounter.Add(1))

	return w, nil
}

type widgets []widget

func (w *widgets) UnmarshalYAML(node *yaml.Node) error {
	var nodes []yaml.Node

	if err := node.Decode(&nodes); err != nil {
		return err
	}

	for _, node := range nodes {
		meta := struct {
			Type string `yaml:"type"`
		}{}

		if err := node.Decode(&meta); err != nil {
			return err
		}

		widget, err := newWidget(meta.Type)
		if err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}

		source, err := sources.NewSource(meta.Type)
		if err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}

		widget.setSource(source)

		if err = node.Decode(widget); err != nil {
			return err
		}

		*w = append(*w, widget)
	}

	return nil
}

type widget interface {
	// These need to be exported because they get called in templates
	Render() template.HTML
	GetType() string
	GetID() uint64

	initialize() error
	setProviders(*widgetProviders)
	update(context.Context)
	setID(uint64)
	handleRequest(w http.ResponseWriter, r *http.Request)
	setHideHeader(bool)
	source() sources.Source
	setSource(sources.Source)
}

type feedEntry struct {
	ID          string
	Title       string
	Description string
	URL         string
	ImageURL    string
	PublishedAt time.Time
}

type cacheType int

const (
	cacheTypeInfinite cacheType = iota
	cacheTypeDuration
	cacheTypeOnTheHour
)

type widgetBase struct {
	ID               uint64           `yaml:"-"`
	Providers        *widgetProviders `yaml:"-"`
	Type             string           `yaml:"type"`
	HideHeader       bool             `yaml:"hide-header"`
	CSSClass         string           `yaml:"css-class"`
	ContentAvailable bool             `yaml:"-"`
	WIP              bool             `yaml:"-"`
	Error            error            `yaml:"-"`
	Notice           error            `yaml:"-"`
	// Source TODO(pulse): Temporary store source on a widget. Later it should be stored in a source registry and only passed to the widget for rendering.
	Source         sources.Source `yaml:"-"`
	templateBuffer bytes.Buffer   `yaml:"-"`
}

type widgetProviders struct {
	assetResolver func(string) string
}

func (w *widgetBase) IsWIP() bool {
	return w.WIP
}

func (w *widgetBase) update(ctx context.Context) {

}

func (w *widgetBase) GetID() uint64 {
	return w.ID
}

func (w *widgetBase) setID(id uint64) {
	w.ID = id
}

func (w *widgetBase) setHideHeader(value bool) {
	w.HideHeader = value
}

func (widget *widgetBase) handleRequest(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (w *widgetBase) GetType() string {
	return w.Type
}

func (w *widgetBase) setProviders(providers *widgetProviders) {
	w.Providers = providers
}

func (w *widgetBase) source() sources.Source {
	return w.Source
}

func (w *widgetBase) setSource(s sources.Source) {
	w.Source = s
}

func (w *widgetBase) Render() template.HTML {
	//TODO(pulse) render the generic widget card
	panic("implement me")
}

func (w *widgetBase) initialize() error {
	//TODO(pulse) implement me
	panic("implement me")
}

func (w *widgetBase) renderTemplate(data any, t *template.Template) template.HTML {
	w.templateBuffer.Reset()
	err := t.Execute(&w.templateBuffer, data)
	if err != nil {
		w.ContentAvailable = false
		w.Error = err

		slog.Error("Failed to render template", "error", err)

		// need to immediately re-render with the error,
		// otherwise risk breaking the page since the widget
		// will likely be partially rendered with tags not closed.
		w.templateBuffer.Reset()
		err2 := t.Execute(&w.templateBuffer, data)

		if err2 != nil {
			slog.Error("Failed to render error within widget", "error", err2, "initial_error", err)
			w.templateBuffer.Reset()
			// TODO: add some kind of a generic widget error template when the widget
			// failed to render, and we also failed to re-render the widget with the error
		}
	}

	return template.HTML(w.templateBuffer.String())
}
