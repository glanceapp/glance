package glance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
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
	widgetByID map[uint64]widget.Widget
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
	BaseURL    string    `yaml:"base-url"`
	AssetsHash string    `yaml:"-"`
	StartedAt  time.Time `yaml:"-"` // used in custom css file
}

type Branding struct {
	HideFooter   bool          `yaml:"hide-footer"`
	CustomFooter template.HTML `yaml:"custom-footer"`
	LogoText     string        `yaml:"logo-text"`
	LogoURL      string        `yaml:"logo-url"`
	FaviconURL   string        `yaml:"favicon-url"`
	PwaText      string        `yaml:"pwa-text"`
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
	Title                 string   `yaml:"name"`
	Slug                  string   `yaml:"slug"`
	Width                 string   `yaml:"width"`
	ShowMobileHeader      bool     `yaml:"show-mobile-header"`
	HideDesktopNavigation bool     `yaml:"hide-desktop-navigation"`
	CenterVertically      bool     `yaml:"center-vertically"`
	Columns               []Column `yaml:"columns"`
	mu                    sync.Mutex
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

func (a *Application) TransformUserDefinedAssetPath(path string) string {
	if strings.HasPrefix(path, "/assets/") {
		return a.Config.Server.BaseURL + path
	}

	return path
}

func NewApplication(config *Config) (*Application, error) {
	if len(config.Pages) == 0 {
		return nil, fmt.Errorf("no pages configured")
	}

	app := &Application{
		Version:    buildVersion,
		Config:     *config,
		slugToPage: make(map[string]*Page),
		widgetByID: make(map[uint64]widget.Widget),
	}

	app.Config.Server.AssetsHash = assets.PublicFSHash
	app.slugToPage[""] = &config.Pages[0]

	providers := &widget.Providers{
		AssetResolver: app.AssetPath,
	}

	for p := range config.Pages {
		if config.Pages[p].Slug == "" {
			config.Pages[p].Slug = titleToSlug(config.Pages[p].Title)
		}

		app.slugToPage[config.Pages[p].Slug] = &config.Pages[p]

		for c := range config.Pages[p].Columns {
			for w := range config.Pages[p].Columns[c].Widgets {
				widget := config.Pages[p].Columns[c].Widgets[w]
				app.widgetByID[widget.GetID()] = widget

				widget.SetProviders(providers)
			}
		}
	}

	config = &app.Config

	config.Server.BaseURL = strings.TrimRight(config.Server.BaseURL, "/")
	config.Theme.CustomCSSFile = app.TransformUserDefinedAssetPath(config.Theme.CustomCSSFile)

	if config.Branding.FaviconURL == "" {
		config.Branding.FaviconURL = app.AssetPath("favicon.png")
	} else {
		config.Branding.FaviconURL = app.TransformUserDefinedAssetPath(config.Branding.FaviconURL)
	}

	config.Branding.LogoURL = app.TransformUserDefinedAssetPath(config.Branding.LogoURL)

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

func (a *Application) HandleWidgetRequest(w http.ResponseWriter, r *http.Request) {
	widgetValue := r.PathValue("widget")

	widgetID, err := strconv.ParseUint(widgetValue, 10, 64)

	if err != nil {
		a.HandleNotFound(w, r)
		return
	}

	widget, exists := a.widgetByID[widgetID]

	if !exists {
		a.HandleNotFound(w, r)
		return
	}

	widget.HandleRequest(w, r)
}

func (a *Application) AssetPath(asset string) string {
	return a.Config.Server.BaseURL + "/static/" + a.Config.Server.AssetsHash + "/" + asset
}

func (a *Application) Serve() error {
	// TODO: add gzip support, static files must have their gzipped contents cached
	// TODO: add HTTPS support
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", a.HandlePageRequest)
	mux.HandleFunc("GET /{page}", a.HandlePageRequest)

	mux.HandleFunc("GET /api/pages/{page}/content/{$}", a.HandlePageContentRequest)
	mux.HandleFunc("/api/widgets/{widget}/{path...}", a.HandleWidgetRequest)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

    mux.HandleFunc(fmt.Sprintf("GET /static/%s/manifest.json", a.Config.Server.AssetsHash), a.HandleManifestRequest)

	mux.Handle(
		fmt.Sprintf("GET /static/%s/{path...}", a.Config.Server.AssetsHash),
		http.StripPrefix("/static/"+a.Config.Server.AssetsHash, FileServerWithCache(http.FS(assets.PublicFS), 24*time.Hour)),
	)

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
	slog.Info("Starting server", "host", a.Config.Server.Host, "port", a.Config.Server.Port, "base-url", a.Config.Server.BaseURL)

	return server.ListenAndServe()
}

func (a *Application) HandleManifestRequest(w http.ResponseWriter, r *http.Request) {
    manifest := map[string]interface{}{
        "name": func() string {
            if a.Config.Branding.PwaText != "" {
                return a.Config.Branding.PwaText
            }
            return "Glance"
        }(),
        "display": "standalone",
        "background_color": "#151519",
        "scope": "/",
        "start_url": "/",
        "icons": []map[string]string{
            {
                "src": func() string {
                    if a.Config.Branding.FaviconURL != "" {
                        return a.Config.Branding.FaviconURL
                    }
                    return "app-icon.png"
                }(),
                "type": "image/png",
                "sizes": "512x512",
            },
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(manifest)
}