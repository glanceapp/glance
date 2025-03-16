package glance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	pageTemplate           = mustParseTemplate("page.html", "document.html")
	pageContentTemplate    = mustParseTemplate("page-content.html")
	pageThemeStyleTemplate = mustParseTemplate("theme-style.gotmpl")
)

type application struct {
	Version          string
	Config           config
	ParsedThemeStyle template.HTML

	slugToPage map[string]*page
	widgetByID map[uint64]widget
}

func newApplication(config *config) (*application, error) {
	app := &application{
		Version:    buildVersion,
		Config:     *config,
		slugToPage: make(map[string]*page),
		widgetByID: make(map[uint64]widget),
	}

	app.slugToPage[""] = &config.Pages[0]

	providers := &widgetProviders{
		assetResolver: app.AssetPath,
	}

	var err error
	app.ParsedThemeStyle, err = executeTemplateToHTML(pageThemeStyleTemplate, &app.Config.Theme)
	if err != nil {
		return nil, fmt.Errorf("parsing theme style: %v", err)
	}

	for p := range config.Pages {
		page := &config.Pages[p]
		page.PrimaryColumnIndex = -1

		if page.Slug == "" {
			page.Slug = titleToSlug(page.Title)
		}

		app.slugToPage[page.Slug] = page

		for c := range page.Columns {
			column := &page.Columns[c]

			if page.PrimaryColumnIndex == -1 && column.Size == "full" {
				page.PrimaryColumnIndex = int8(c)
			}

			for w := range column.Widgets {
				widget := column.Widgets[w]
				app.widgetByID[widget.GetID()] = widget

				widget.setProviders(providers)
			}
		}
	}

	config = &app.Config

	config.Server.BaseURL = strings.TrimRight(config.Server.BaseURL, "/")
	config.Theme.CustomCSSFile = app.transformUserDefinedAssetPath(config.Theme.CustomCSSFile)

	if config.Branding.FaviconURL == "" {
		config.Branding.FaviconURL = app.AssetPath("favicon.png")
	} else {
		config.Branding.FaviconURL = app.transformUserDefinedAssetPath(config.Branding.FaviconURL)
	}

	config.Branding.LogoURL = app.transformUserDefinedAssetPath(config.Branding.LogoURL)

	return app, nil
}

func (p *page) updateOutdatedWidgets() {
	now := time.Now()

	var wg sync.WaitGroup
	context := context.Background()

	for c := range p.Columns {
		for w := range p.Columns[c].Widgets {
			widget := p.Columns[c].Widgets[w]

			if !widget.requiresUpdate(&now) {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				widget.update(context)
			}()
		}
	}

	wg.Wait()
}

func (a *application) transformUserDefinedAssetPath(path string) string {
	if strings.HasPrefix(path, "/assets/") {
		return a.Config.Server.BaseURL + path
	}

	return path
}

type pageTemplateData struct {
	App  *application
	Page *page
}

func (a *application) handlePageRequest(w http.ResponseWriter, r *http.Request) {
	page, exists := a.slugToPage[r.PathValue("page")]

	if !exists {
		a.handleNotFound(w, r)
		return
	}

	pageData := pageTemplateData{
		Page: page,
		App:  a,
	}

	var responseBytes bytes.Buffer
	err := pageTemplate.Execute(&responseBytes, pageData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Write(responseBytes.Bytes())
}

func (a *application) handlePageContentRequest(w http.ResponseWriter, r *http.Request) {
	page, exists := a.slugToPage[r.PathValue("page")]

	if !exists {
		a.handleNotFound(w, r)
		return
	}

	pageData := pageTemplateData{
		Page: page,
	}

	var err error
	var responseBytes bytes.Buffer

	func() {
		page.mu.Lock()
		defer page.mu.Unlock()

		page.updateOutdatedWidgets()
		err = pageContentTemplate.Execute(&responseBytes, pageData)
	}()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Write(responseBytes.Bytes())
}

func (a *application) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	// TODO: add proper not found page
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Page not found"))
}

func (a *application) handleWidgetRequest(w http.ResponseWriter, r *http.Request) {
	widgetValue := r.PathValue("widget")

	widgetID, err := strconv.ParseUint(widgetValue, 10, 64)
	if err != nil {
		a.handleNotFound(w, r)
		return
	}

	widget, exists := a.widgetByID[widgetID]

	if !exists {
		a.handleNotFound(w, r)
		return
	}

	widget.handleRequest(w, r)
}

func (a *application) AssetPath(asset string) string {
	return a.Config.Server.BaseURL + "/static/" + staticFSHash + "/" + asset
}

func (a *application) server() (func() error, func() error) {
	// TODO: add gzip support, static files must have their gzipped contents cached
	// TODO: add HTTPS support
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", a.handlePageRequest)
	mux.HandleFunc("GET /{page}", a.handlePageRequest)

	mux.HandleFunc("GET /api/pages/{page}/content/{$}", a.handlePageContentRequest)
	mux.HandleFunc("/api/widgets/{widget}/{path...}", a.handleWidgetRequest)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc(fmt.Sprintf("GET /static/%s/manifest.json", staticFSHash), a.handleManifestRequest)
	mux.Handle(
		fmt.Sprintf("GET /static/%s/{path...}", staticFSHash),
		http.StripPrefix("/static/"+staticFSHash, fileServerWithCache(http.FS(staticFS), 24*time.Hour)),
	)

	var absAssetsPath string
	if a.Config.Server.AssetsPath != "" {
		absAssetsPath, _ = filepath.Abs(a.Config.Server.AssetsPath)
		assetsFS := fileServerWithCache(http.Dir(a.Config.Server.AssetsPath), 2*time.Hour)
		mux.Handle("/assets/{path...}", http.StripPrefix("/assets/", assetsFS))
	}

	server := http.Server{
		Addr:    fmt.Sprintf("%s:%d", a.Config.Server.Host, a.Config.Server.Port),
		Handler: mux,
	}

	start := func() error {
		a.Config.Server.StartedAt = time.Now()
		log.Printf("Starting server on %s:%d (base-url: \"%s\", assets-path: \"%s\")\n",
			a.Config.Server.Host,
			a.Config.Server.Port,
			a.Config.Server.BaseURL,
			absAssetsPath,
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}

		return nil
	}

	stop := func() error {
		return server.Close()
	}

	return start, stop
}

func (a *application) handleManifestRequest(w http.ResponseWriter, r *http.Request) {
	manifest := map[string]interface{}{
		"name": func() string {
			if a.Config.Branding.PwaText != "" {
				return a.Config.Branding.PwaText
			}
			return "Glance"
		}(),
		"short_name":       a.Config.Branding.PwaText,
		"display":          "standalone",
		"background_color": "#151519",
		"scope":            "/",
		"start_url":        "/",
		"icons": []map[string]string{
			{
				"src": func() string {
					if a.Config.Branding.FaviconURL != "" {
						return a.transformUserDefinedAssetPath(a.Config.Branding.FaviconURL)
					}
					return a.AssetPath("app-icon.png")
				}(),
				"type":    "image/png",
				"sizes":   "512x512",
				"purpose": "any maskable",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(manifest)
}
