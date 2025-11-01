package glance

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"os"
	"strconv"
)

var todoWidgetTemplate = mustParseTemplate("todo.html", "widget-base.html")

type todoWidget struct {
	widgetBase  `yaml:",inline"`
	cachedHTML  template.HTML `yaml:"-"`
	TodoID      string        `yaml:"id"`
	StorageType string        `yaml:"storage-type"`
}

func (widget *todoWidget) initialize() error {
	widget.withTitle("To-do").withError(nil)

	switch widget.StorageType {
	case "server":
		if widget.TodoID == "" {
			widget.TodoID = strconv.FormatUint(widget.GetID(), 10)
		}
	case "browser":
	default:
		widget.StorageType = "browser"
	}

	widget.cachedHTML = widget.renderTemplate(widget, todoWidgetTemplate)

	return nil
}

func (widget *todoWidget) getHandlerFunc() map[string]http.HandlerFunc {
	if widget.StorageType == "server" {
		return map[string]http.HandlerFunc{
			"GET /{id}": widget.handleGet,
			"PUT /{id}": widget.handlePut,
		}
	}

	return nil
}

func (widget *todoWidget) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := loadFile(widget.GetType(), id+".json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (widget *todoWidget) handlePut(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var todos []map[string]any
	if err := json.NewDecoder(r.Body).Decode(&todos); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := json.Marshal(todos)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := saveFile(widget.GetType(), id+".json", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{}"))
}

func (widget *todoWidget) Render() template.HTML {
	return widget.cachedHTML
}
