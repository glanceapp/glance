package glance

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/afero"
)

var todoWidgetTemplate = mustParseTemplate("todo.html", "widget-base.html")

type todoWidget struct {
	widgetBase  `yaml:",inline"`
	cachedHTML  template.HTML `yaml:"-"`
	TodoID      string        `yaml:"id"`
	StorageType string        `yaml:"storage-type"`
	storage     afero.Fs      `yaml:"-"`
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

func (widget *todoWidget) handleRequest(w http.ResponseWriter, r *http.Request) {
	filepath := fmt.Sprintf("%s.json", r.PathValue("id"))

	switch r.Method {
	case http.MethodGet:
		f, err := widget.storage.Open(filepath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if fi.IsDir() {
			http.Error(w, fmt.Errorf("not a file: %s", filepath).Error(), http.StatusInternalServerError)
			return
		}

		http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)

	case http.MethodPut:
		var todos []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&todos); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		f, err := widget.storage.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if err := json.NewEncoder(f).Encode(todos); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (widget *todoWidget) Render() template.HTML {
	return widget.cachedHTML
}
