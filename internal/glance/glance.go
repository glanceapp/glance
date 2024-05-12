package glance

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/widget"
)

var buildVersion = "dev"

var sequentialWhitespacePattern = regexp.MustCompile(`\s+`)

type Application struct {
	Version    string
	Config     Config
	slugToPage map[string]*Page
}

type Theme struct {
	BackgroundColor          *widget.HSLColorField `yaml:"background-color"`
	PrimaryColor             *widget.HSLColorField `yaml:"primary-color"`
	PositiveColor            *widget.HSLColorField `yaml:"positive-color"`
	NegativeColor            *widget.HSLColorField `yaml:"negative-color"`
	Light                    bool                  `yaml:"light"`
	ContrastMultiplier       float32               `yaml:"contrast-multiplier"`
	TextSaturationMultiplier float32               `yaml:"text-saturation-multiplier"`
	CustomCSSFile            string                `yaml:"custom-css-file"`
}

type Server struct {
	Host       string    `yaml:"host"`
	Port       uint16    `yaml:"port"`
	AssetsPath string    `yaml:"assets-path"`
	StartedAt  time.Time `yaml:"-"`
}

type Column struct {
	Size    string         `yaml:"size"`
	Widgets widget.Widgets `yaml:"widgets"`
}

type templateData struct {
	App  *Application
	Page *Page
}

type Page struct {
	Title            string   `yaml:"name"`
	Slug             string   `yaml:"slug"`
	ShowMobileHeader bool     `yaml:"show-mobile-header"`
	Columns          []Column `yaml:"columns"`
	mu               sync.Mutex
}

func (p *Page) UpdateOutdatedWidgets() {
	now := time.Now()

	var wg sync.WaitGroup
	context := context.Background()

	for c := range p.Columns {
		for w := range p.Columns[c].Widgets {
			widget := p.Columns[c].Widgets[w]

			if !widget.RequiresUpdate(&now) {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				widget.Update(context)
			}()
		}
	}

	wg.Wait()
}

// TODO: fix, currently very simple, lots of uncovered edge cases
func titleToSlug(s string) string {
	s = strings.ToLower(s)
	s = sequentialWhitespacePattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	return s
}

func NewApplication(config *Config) (*Application, error) {
	if len(config.Pages) == 0 {
		return nil, fmt.Errorf("no pages configured")
	}

	app := &Application{
		Version:    buildVersion,
		Config:     *config,
		slugToPage: make(map[string]*Page),
	}

	app.slugToPage[""] = &config.Pages[0]

	for i := range config.Pages {
		if config.Pages[i].Slug == "" {
			config.Pages[i].Slug = titleToSlug(config.Pages[i].Title)
		}

		app.slugToPage[config.Pages[i].Slug] = &config.Pages[i]
	}

	return app, nil
}

func (a *Application) HandlePageRequest(w http.ResponseWriter, r *http.Request) {
	page, exists := a.slugToPage[r.PathValue("page")]

	if !exists {
		a.HandleNotFound(w, r)
		return
	}

	pageData := templateData{
		Page: page,
		App:  a,
	}

	var responseBytes bytes.Buffer
	err := assets.PageTemplate.Execute(&responseBytes, pageData)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Write(responseBytes.Bytes())
}

func (a *Application) HandlePageContentRequest(w http.ResponseWriter, r *http.Request) {
	page, exists := a.slugToPage[r.PathValue("page")]

	if !exists {
		a.HandleNotFound(w, r)
		return
	}

	pageData := templateData{
		Page: page,
	}

	page.mu.Lock()
	defer page.mu.Unlock()
	page.UpdateOutdatedWidgets()

	var responseBytes bytes.Buffer
	err := assets.PageContentTemplate.Execute(&responseBytes, pageData)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Write(responseBytes.Bytes())
}

func (a *Application) HandleNotFound(w http.ResponseWriter, r *http.Request) {
	// TODO: add proper not found page
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Page not found"))
}

func FileServerWithCache(fs http.FileSystem, cacheDuration time.Duration) http.Handler {
	server := http.FileServer(fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: fix always setting cache control even if the file doesn't exist
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(cacheDuration.Seconds())))
		server.ServeHTTP(w, r)
	})
}

func (a *Application) Serve() error {
	// TODO: add gzip support, static files must have their gzipped contents cached
	// TODO: add HTTPS support
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", a.HandlePageRequest)
	mux.HandleFunc("GET /{page}", a.HandlePageRequest)
	mux.HandleFunc("GET /api/pages/{page}/content/{$}", a.HandlePageContentRequest)
	mux.Handle("GET /static/{path...}", http.StripPrefix("/static/", FileServerWithCache(http.FS(assets.PublicFS), 2*time.Hour)))

	if a.Config.Server.AssetsPath != "" {
		absAssetsPath, err := filepath.Abs(a.Config.Server.AssetsPath)

		if err != nil {
			return fmt.Errorf("invalid assets path: %s", a.Config.Server.AssetsPath)
		}

		slog.Info("Serving assets", "path", absAssetsPath)
		assetsFS := FileServerWithCache(http.Dir(a.Config.Server.AssetsPath), 2*time.Hour)
		mux.Handle("/assets/{path...}", http.StripPrefix("/assets/", assetsFS))
	}

	server := http.Server{
		Addr:    fmt.Sprintf("%s:%d", a.Config.Server.Host, a.Config.Server.Port),
		Handler: mux,
	}

	a.Config.Server.StartedAt = time.Now()

	slog.Info("Starting server", "host", a.Config.Server.Host, "port", a.Config.Server.Port)
	return server.ListenAndServe()
}
