package dynacat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

var (
	pageTemplate        = mustParseTemplate("page.html", "document.html", "footer.html")
	pageContentTemplate = mustParseTemplate("page-content.html")
	manifestTemplate    = mustParseTemplate("manifest.json")
)

const STATIC_ASSETS_CACHE_DURATION = 24 * time.Hour
const REMOTE_IMAGE_CACHE_DURATION = 7 * 24 * time.Hour

var reservedPageSlugs = []string{"login", "logout", "callback"}

type imageProxyInfo struct {
	URL           string
	AllowInsecure bool
}

type application struct {
	Version   string
	CreatedAt time.Time
	Config    config

	parsedManifest []byte

	slugToPage   map[string]*page
	widgetByID   map[uint64]widget
	widgetToPage map[uint64]*page

	RequiresAuth           bool
	OIDCEnabled            bool
	PasswordEnabled        bool
	authSecretKey          []byte
	usernameHashToUsername map[string]string
	authAttemptsMu         sync.Mutex
	failedAuthAttempts     map[string]*failedAuthAttempt

	oidcProvider *gooidc.Provider
	oidcVerifier *gooidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	oidcSessions *sessionStore

	todoStorage      *todoStorage
	todoListIDToPage map[string]*page

	sseMu                sync.RWMutex
	sseClients           map[*sseClient]struct{}
	DynamicUpdateEnabled bool

	imageProxyMu   sync.RWMutex
	imageProxyURLs map[string]imageProxyInfo

	imageCache *imageCache
}

func newApplication(c *config) (*application, error) {
	app := &application{
		Version:        buildVersion,
		CreatedAt:      time.Now(),
		Config:         *c,
		slugToPage:     make(map[string]*page),
		widgetByID:     make(map[uint64]widget),
		widgetToPage:   make(map[uint64]*page),
		sseClients:       make(map[*sseClient]struct{}),
		imageProxyURLs:   make(map[string]imageProxyInfo),
		todoListIDToPage: make(map[string]*page),
	}
	config := &app.Config

	//
	// Init auth
	//

	hasAnyAuth := len(config.Auth.Users) > 0 || config.Auth.OIDC != nil
	if hasAnyAuth {
		secretBytes, err := base64.StdEncoding.DecodeString(config.Auth.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("decoding secret-key: %v", err)
		}

		if len(secretBytes) != AUTH_SECRET_KEY_LENGTH {
			return nil, fmt.Errorf("secret-key must be exactly %d bytes", AUTH_SECRET_KEY_LENGTH)
		}

		app.authSecretKey = secretBytes
		app.failedAuthAttempts = make(map[string]*failedAuthAttempt)

		requireAuth := true
		if config.Auth.RequireAuth != nil {
			requireAuth = *config.Auth.RequireAuth
		}
		app.RequiresAuth = requireAuth
	}

	if len(config.Auth.Users) > 0 && !config.Auth.DisablePassword {
		app.PasswordEnabled = true
		app.usernameHashToUsername = make(map[string]string)

		for username := range config.Auth.Users {
			user := config.Auth.Users[username]
			usernameHash, err := computeUsernameHash(username, app.authSecretKey)
			if err != nil {
				return nil, fmt.Errorf("computing username hash for user %s: %v", username, err)
			}
			app.usernameHashToUsername[string(usernameHash)] = username

			if user.PasswordHashString != "" {
				user.PasswordHash = []byte(user.PasswordHashString)
				user.PasswordHashString = ""
			} else {
				hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
				if err != nil {
					return nil, fmt.Errorf("hashing password for user %s: %v", username, err)
				}

				user.Password = ""
				user.PasswordHash = hashedPassword
			}
		}
	}

	if config.Auth.OIDC != nil {
		provider, verifier, oauth2Cfg, err := initOIDCProvider(config.Auth.OIDC)
		if err != nil {
			return nil, fmt.Errorf("initializing OIDC: %v", err)
		}
		app.oidcProvider = provider
		app.oidcVerifier = verifier
		app.oauth2Config = oauth2Cfg
		app.oidcSessions = newSessionStore()
		app.OIDCEnabled = true
	}

	//
	// Init themes
	//

	if !config.Theme.DisablePicker {
		themeKeys := make([]string, 0, 2)
		themeProps := make([]*themeProperties, 0, 2)

		defaultDarkTheme, ok := config.Theme.Presets.Get("default-dark")
		if ok && !config.Theme.SameAs(defaultDarkTheme) || !config.Theme.SameAs(&themeProperties{}) {
			themeKeys = append(themeKeys, "default-dark")
			themeProps = append(themeProps, &themeProperties{})
		}

		themeKeys = append(themeKeys, "default-light")
		themeProps = append(themeProps, &themeProperties{
			Light:                    true,
			BackgroundColor:          &hslColorField{240, 13, 95},
			PrimaryColor:             &hslColorField{230, 100, 30},
			NegativeColor:            &hslColorField{0, 70, 50},
			ContrastMultiplier:       1.3,
			TextSaturationMultiplier: 0.5,
		})

		themePresets, err := newOrderedYAMLMap(themeKeys, themeProps)
		if err != nil {
			return nil, fmt.Errorf("creating theme presets: %v", err)
		}
		config.Theme.Presets = *themePresets.Merge(&config.Theme.Presets)

		for key, properties := range config.Theme.Presets.Items() {
			properties.Key = key
			if err := properties.init(); err != nil {
				return nil, fmt.Errorf("initializing preset theme %s: %v", key, err)
			}
		}
	}

	config.Theme.Key = "default"
	if err := config.Theme.init(); err != nil {
		return nil, fmt.Errorf("initializing default theme: %v", err)
	}

	config.Server.BaseURL = strings.TrimRight(config.Server.BaseURL, "/")
	if config.Server.CacheDir == "" {
		config.Server.CacheDir = ".cache"
	}
	cacheDir := config.Server.CacheDir
	if !filepath.IsAbs(cacheDir) {
		absCacheDir, err := filepath.Abs(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("resolving cache-dir: %v", err)
		}
		cacheDir = absCacheDir
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache-dir: %v", err)
	}
	config.Server.CacheDir = cacheDir

	for _, cidr := range config.Server.TrustedProxies {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if !strings.Contains(cidr, "/") {
			if ip := net.ParseIP(cidr); ip != nil {
				if ip.To4() != nil {
					cidr = cidr + "/32"
				} else {
					cidr = cidr + "/128"
				}
			}
		}
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted-proxies entry %q: %v", cidr, err)
		}
		config.Server.trustedProxyNets = append(config.Server.trustedProxyNets, ipnet)
	}

	//
	// Init pages
	//

	app.slugToPage[""] = &config.Pages[0]

	dynamicUpdateEnabled := true
	if v := os.Getenv("ENABLE_DYNAMIC_UPDATE"); v == "false" || v == "0" || v == "f" {
		dynamicUpdateEnabled = false
	}

	app.DynamicUpdateEnabled = dynamicUpdateEnabled

	app.imageCache = newImageCache(config.Server.BaseURL, config.Server.CacheDir)

	providers := &widgetProviders{
		assetResolver:        app.StaticAssetPath,
		imageCache:           app.imageCache,
		baseURL:              config.Server.BaseURL,
		DynamicUpdateEnabled: dynamicUpdateEnabled,
		app:                  app,
	}

	for p := range config.Pages {
		page := &config.Pages[p]
		page.PrimaryColumnIndex = -1

		if page.Slug == "" {
			page.Slug = titleToSlug(page.Title)
		}

		if slices.Contains(reservedPageSlugs, page.Slug) {
			return nil, fmt.Errorf("page slug \"%s\" is reserved", page.Slug)
		}

		app.slugToPage[page.Slug] = page

		if page.Width == "default" {
			page.Width = ""
		}

		if page.DesktopNavigationWidth == "" && page.DesktopNavigationWidth != "default" {
			page.DesktopNavigationWidth = page.Width
		}

		var collectTodoListIDs func(ws widgets)
		collectTodoListIDs = func(ws widgets) {
			for _, w := range ws {
				switch v := w.(type) {
				case *todoWidget:
					if v.Storage == "server" && v.TodoID != "" {
						app.todoListIDToPage[v.TodoID] = page
					}
				case *groupWidget:
					collectTodoListIDs(v.Widgets)
				case *splitColumnWidget:
					collectTodoListIDs(v.Widgets)
				}
			}
		}

		registerWidget := func(widget widget) {
			app.widgetByID[widget.GetID()] = widget
			app.widgetToPage[widget.GetID()] = page
			widget.setProviders(providers)
		}

		for i := range page.HeadWidgets {
			registerWidget(page.HeadWidgets[i])
		}
		collectTodoListIDs(page.HeadWidgets)

		for c := range page.Columns {
			column := &page.Columns[c]

			if page.PrimaryColumnIndex == -1 && column.Size == "full" {
				page.PrimaryColumnIndex = int8(c)
			}

			for w := range column.Widgets {
				registerWidget(column.Widgets[w])
			}
			collectTodoListIDs(column.Widgets)
		}
	}

	config.Theme.CustomCSSFile = app.resolveUserDefinedAssetPath(config.Theme.CustomCSSFile)
	config.Branding.LogoURL = app.resolveUserDefinedAssetPath(config.Branding.LogoURL)

	config.Branding.FaviconURL = ternary(
		config.Branding.FaviconURL == "",
		app.StaticAssetPath("favicon.svg"),
		app.resolveUserDefinedAssetPath(config.Branding.FaviconURL),
	)

	config.Branding.FaviconType = ternary(
		strings.HasSuffix(config.Branding.FaviconURL, ".svg"),
		"image/svg+xml",
		"image/png",
	)

	if config.Branding.AppName == "" {
		config.Branding.AppName = "Dynacat"
	}

	if config.Branding.AppIconURL == "" {
		config.Branding.AppIconURL = app.StaticAssetPath("app-icon.svg")
	}

	if config.Branding.AppBackgroundColor == "" {
		config.Branding.AppBackgroundColor = config.Theme.BackgroundColorAsHex
	}

	manifest, err := executeTemplateToString(manifestTemplate, templateData{App: app})
	if err != nil {
		return nil, fmt.Errorf("parsing manifest.json: %v", err)
	}
	app.parsedManifest = []byte(manifest)

	//
	// Init todo storage
	//

	needsTodoDB := false
	for p := range config.Pages {
		for _, w := range config.Pages[p].HeadWidgets {
			if tw, ok := w.(*todoWidget); ok && tw.Storage == "server" {
				needsTodoDB = true
				break
			}
		}
		if needsTodoDB {
			break
		}
		for c := range config.Pages[p].Columns {
			for _, w := range config.Pages[p].Columns[c].Widgets {
				if tw, ok := w.(*todoWidget); ok && tw.Storage == "server" {
					needsTodoDB = true
					break
				}
			}
			if needsTodoDB {
				break
			}
		}
	}

	if needsTodoDB {
		dbPath := config.Server.DBPath
		if dbPath == "" {
			dbPath = "/app/assets/dynacat.db"
		}
		app.todoStorage = newTodoStorage(dbPath)
	}

	return app, nil
}

func (a *application) sseRegisterClient(c *sseClient) {
	a.sseMu.Lock()
	a.sseClients[c] = struct{}{}
	a.sseMu.Unlock()
}

func (a *application) sseUnregisterClient(c *sseClient) {
	a.sseMu.Lock()
	delete(a.sseClients, c)
	a.sseMu.Unlock()
}

func (a *application) sseBroadcast(msg string) {
	a.sseMu.RLock()
	defer a.sseMu.RUnlock()
	for c := range a.sseClients {
		select {
		case c.ch <- msg:
		default: // client too slow; drop rather than block
		}
	}
}

func (p *page) updateOutdatedWidgets() {
	now := time.Now()

	var wg sync.WaitGroup
	context := context.Background()

	for w := range p.HeadWidgets {
		widget := p.HeadWidgets[w]

		if !widget.requiresUpdate(&now) {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			widget.update(context)
		}()
	}

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

func (p *page) GetMinUpdateInterval() int64 {
	if !p.DynamicUpdatesEnabled() {
		return 0
	}

	min, found := getMinUpdateIntervalForWidgets(p.HeadWidgets)

	for c := range p.Columns {
		m, f := getMinUpdateIntervalForWidgets(p.Columns[c].Widgets)
		if f {
			if !found || m < min {
				min = m
				found = true
			}
		}
	}

	if !found {
		return 0
	}

	return min.Milliseconds()
}

func getMinUpdateIntervalForWidgets(ws widgets) (time.Duration, bool) {
	min := 1 * time.Second
	found := false

	for _, w := range ws {
		var interval time.Duration
		widgetFound := false

		if cw, ok := w.(*customAPIWidget); ok {
			// Only include custom-api widgets in global polling if they don't have update-interval set
			// Widgets with update-interval will poll independently on the client side
			if cw.UpdateInterval == nil {
				widgetFound = true
				interval = 1 * time.Second
			}
		} else if group, ok := w.(*groupWidget); ok {
			interval, widgetFound = getMinUpdateIntervalForWidgets(group.Widgets)
		} else if sc, ok := w.(*splitColumnWidget); ok {
			interval, widgetFound = getMinUpdateIntervalForWidgets(sc.Widgets)
		}

		if widgetFound {
			if !found || interval < min {
				min = interval
			}
			found = true
		}
	}

	return min, found
}

func (a *application) resolveUserDefinedAssetPath(path string) string {
	if strings.HasPrefix(path, "/assets/") {
		return a.Config.Server.BaseURL + path
	}

	return path
}

type templateRequestData struct {
	Theme *themeProperties
}

type templateData struct {
	App             *application
	Page            *page
	Request         templateRequestData
	AuthUser        *authenticatedUser
	AccessiblePages []*page
	OIDCError       string
}

func (a *application) populateTemplateRequestData(data *templateRequestData, r *http.Request) {
	theme := &a.Config.Theme.themeProperties

	if !a.Config.Theme.DisablePicker {
		selectedTheme, err := r.Cookie("theme")
		if err == nil {
			preset, exists := a.Config.Theme.Presets.Get(selectedTheme.Value)
			if exists {
				theme = preset
			}
		}
	}

	data.Theme = theme
}

func (a *application) getAccessiblePages(user *authenticatedUser) []*page {
	pages := make([]*page, 0, len(a.Config.Pages))
	for i := range a.Config.Pages {
		p := &a.Config.Pages[i]
		if user == nil {
			// Unauthenticated: only pages with no restrictions (when RequireAuth is false)
			if len(p.AllowedUsers) == 0 && len(p.AllowedGroups) == 0 {
				pages = append(pages, p)
			}
		} else {
			if a.isUserAllowedOnPage(user, p) {
				pages = append(pages, p)
			}
		}
	}
	return pages
}

func (a *application) handlePageRequest(w http.ResponseWriter, r *http.Request) {
	page, exists := a.slugToPage[r.PathValue("page")]
	if !exists {
		a.handleNotFound(w, r)
		return
	}

	if a.handleAccessControl(w, r, page, redirectToLogin) {
		return
	}

	user := a.getAuthenticatedUser(w, r)
	data := templateData{
		Page:            page,
		App:             a,
		AuthUser:        user,
		AccessiblePages: a.getAccessiblePages(user),
	}
	a.populateTemplateRequestData(&data.Request, r)

	var responseBytes bytes.Buffer
	err := pageTemplate.Execute(&responseBytes, data)
	if err != nil {
		log.Printf("rendering page template: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
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

	if a.handleAccessControl(w, r, page, showUnauthorizedJSON) {
		return
	}

	pageData := templateData{
		Page: page,
	}

	var err error
	var responseBytes bytes.Buffer
	isCacheBuilding := false

	func() {
		page.mu.Lock()
		defer page.mu.Unlock()

		// Determine cache-build status after widgets have had a chance to queue
		// image fetches to avoid missing the initial "building cache" response.
		page.updateOutdatedWidgets()
		if a.imageCache != nil {
			isCacheBuilding = a.imageCache.IsBuildingCache()
		}
		err = pageContentTemplate.Execute(&responseBytes, pageData)
	}()

	w.Header().Set("X-Dynacat-Cache-Building", strconv.FormatBool(isCacheBuilding))

	if err != nil {
		log.Printf("rendering page content template: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
		return
	}

	w.Write(responseBytes.Bytes())
}

func (a *application) addressOfRequest(r *http.Request) string {
	remoteAddrWithoutPort := func() string {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			return host
		}
		return r.RemoteAddr
	}

	if !a.Config.Server.Proxied {
		return remoteAddrWithoutPort()
	}

	remote := remoteAddrWithoutPort()
	trustedNets := a.Config.Server.trustedProxyNets

	// Without a trusted-proxies allow-list, honoring XFF would let clients spoof.
	if len(trustedNets) == 0 {
		return remote
	}

	ipIsTrusted := func(ipStr string) bool {
		ip := net.ParseIP(strings.TrimSpace(ipStr))
		if ip == nil {
			return false
		}
		for _, n := range trustedNets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}

	if !ipIsTrusted(remote) {
		return remote
	}

	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor == "" {
		return remote
	}

	ips := strings.Split(forwardedFor, ",")
	// Walk right-to-left, skipping trusted proxies; return the first untrusted hop.
	for i := len(ips) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(ips[i])
		if candidate == "" {
			continue
		}
		if ipIsTrusted(candidate) {
			continue
		}
		return candidate
	}

	return remote
}

func (a *application) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	// TODO: add proper not found page
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Page not found"))
}

func (a *application) handleWidgetContentRequest(w http.ResponseWriter, r *http.Request) {
	if a.handleUnauthorizedResponse(w, r, showUnauthorizedJSON) {
		return
	}

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

	page, exists := a.widgetToPage[widgetID]
	if !exists {
		a.handleNotFound(w, r)
		return
	}

	if a.handleAccessControl(w, r, page, showUnauthorizedJSON) {
		return
	}

	page.mu.Lock()
	defer page.mu.Unlock()

	widget.update(context.Background())

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(widget.Render()))
}

func (a *application) handleWidgetActionRequest(w http.ResponseWriter, r *http.Request) {
	if a.handleUnauthorizedResponse(w, r, showUnauthorizedJSON) {
		return
	}

	widgetID, err := strconv.ParseUint(r.PathValue("widget"), 10, 64)
	if err != nil {
		http.Error(w, "invalid widget", http.StatusBadRequest)
		return
	}

	widget, exists := a.widgetByID[widgetID]
	if !exists {
		a.handleNotFound(w, r)
		return
	}

	page, exists := a.widgetToPage[widgetID]
	if !exists {
		a.handleNotFound(w, r)
		return
	}

	if a.handleAccessControl(w, r, page, showUnauthorizedJSON) {
		return
	}

	widget.handleRequest(w, r)
}

func (a *application) StaticAssetPath(asset string) string {
	return a.Config.Server.BaseURL + "/static/" + staticFSHash + "/" + asset
}

func (a *application) VersionedAssetPath(asset string) string {
	return a.Config.Server.BaseURL + asset +
		"?v=" + strconv.FormatInt(a.CreatedAt.Unix(), 10)
}

const todoMaxBodyBytes = 1 << 20 // 1 MiB

func (a *application) authorizeTodoRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	listID := r.PathValue("listID")
	pg, exists := a.todoListIDToPage[listID]
	if !exists {
		a.handleNotFound(w, r)
		return "", false
	}
	if a.handleAccessControl(w, r, pg, showUnauthorizedJSON) {
		return "", false
	}
	return listID, true
}

func (a *application) handleTodoLoad(w http.ResponseWriter, r *http.Request) {
	listID, ok := a.authorizeTodoRequest(w, r)
	if !ok {
		return
	}

	tasks, err := a.todoStorage.loadTasks(listID)
	if err != nil {
		log.Printf("todo load: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (a *application) handleTodoSave(w http.ResponseWriter, r *http.Request) {
	listID, ok := a.authorizeTodoRequest(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, todoMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var tasks []todoTask
	if err := json.Unmarshal(body, &tasks); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if err := a.todoStorage.saveTasks(listID, tasks); err != nil {
		log.Printf("todo save: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *application) securityHeadersMiddleware(next http.Handler) http.Handler {
	frameAncestors := "'self'"
	xFrameOptions := "SAMEORIGIN"
	if len(a.Config.Server.AllowedEmbedHosts) > 0 {
		frameAncestors = "'self' " + strings.Join(a.Config.Server.AllowedEmbedHosts, " ")
		xFrameOptions = "ALLOW-FROM " + strings.Join(a.Config.Server.AllowedEmbedHosts, " ")
	}
	csp := "default-src 'self'; " +
		"img-src 'self' data: blob: https: http:; " +
		"media-src 'self' data: blob: https: http:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"font-src 'self' data:; " +
		"connect-src 'self' https: http:; " +
		"frame-src *; " +
		"frame-ancestors " + frameAncestors + "; " +
		"base-uri 'self'; " +
		"form-action 'self'"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", xFrameOptions)
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", csp)
		if a.Config.Server.HTTPS || a.isRequestHTTPS(r) {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *application) isRequestHTTPS(r *http.Request) bool {
	if a.Config.Server.HTTPS {
		return true
	}
	if r.TLS != nil {
		return true
	}
	if a.Config.Server.Proxied {
		return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	}
	return false
}

func (a *application) server() (func() error, func() error) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", a.handlePageRequest)
	mux.HandleFunc("GET /{page}", a.handlePageRequest)

	mux.HandleFunc("GET /api/pages/{page}/content/{$}", a.handlePageContentRequest)

	if !a.Config.Theme.DisablePicker {
		mux.HandleFunc("POST /api/set-theme/{key}", a.handleThemeChangeRequest)
	}

	mux.HandleFunc("GET /api/widgets/{widget}/content/{$}", a.handleWidgetContentRequest)
	mux.HandleFunc("POST /api/widgets/{widget}/action/{action...}", a.handleWidgetActionRequest)
	mux.HandleFunc("GET /api/sse/updates", a.handleSSEUpdates)
	mux.HandleFunc("GET /api/image-proxy/{hash}", a.handleImageProxyRequest)
	mux.HandleFunc("GET /api/search/autocomplete", a.handleSearchAutocompleteRequest)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if a.AnyAuthEnabled() {
		mux.HandleFunc("GET /login", a.handleLoginPageRequest)
		mux.HandleFunc("GET /logout", a.handleLogoutRequest)
	}

	if a.PasswordEnabled {
		mux.HandleFunc("POST /api/authenticate", a.handleAuthenticationAttempt)
	}

	if a.OIDCEnabled {
		mux.HandleFunc("GET /api/oidc/login", a.handleOIDCLogin)
		mux.HandleFunc("GET /api/oidc/callback", a.handleOIDCCallback)
	}

	if a.todoStorage != nil {
		mux.HandleFunc("GET /api/todo/{listID}", a.handleTodoLoad)
		mux.HandleFunc("PUT /api/todo/{listID}", a.handleTodoSave)
	}

	mux.Handle(
		fmt.Sprintf("GET /static/%s/{path...}", staticFSHash),
		http.StripPrefix(
			"/static/"+staticFSHash,
			fileServerWithCache(http.FS(staticFS), STATIC_ASSETS_CACHE_DURATION),
		),
	)

	if a.Config.Server.CacheDir != "" {
		cacheHandler := http.StripPrefix(
			"/.cache",
			fileServerWithCache(http.Dir(a.Config.Server.CacheDir), REMOTE_IMAGE_CACHE_DURATION),
		)

		if a.RequiresAuth {
			mux.Handle("GET /.cache/{path...}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if a.handleUnauthorizedResponse(w, r, showUnauthorizedJSON) {
					return
				}

				cacheHandler.ServeHTTP(w, r)
			}))
		} else {
			mux.Handle("GET /.cache/{path...}", cacheHandler)
		}
	}

	assetCacheControlValue := fmt.Sprintf(
		"public, max-age=%d",
		int(STATIC_ASSETS_CACHE_DURATION.Seconds()),
	)

	mux.HandleFunc(fmt.Sprintf("GET /static/%s/css/bundle.css", staticFSHash), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", assetCacheControlValue)
		w.Header().Add("Content-Type", "text/css; charset=utf-8")
		w.Write(bundledCSSContents)
	})

	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", assetCacheControlValue)
		w.Header().Add("Content-Type", "application/json")
		w.Write(a.parsedManifest)
	})

	assetsPath := a.Config.Server.AssetsPath
	if assetsPath == "" {
		assetsPath = "/app/assets"
	}

	absAssetsPath, _ := filepath.Abs(assetsPath)
	assetsFS := fileServerWithCache(http.Dir(assetsPath), 2*time.Hour)
	assetsHandler := http.StripPrefix("/assets/", assetsFS)
	if a.RequiresAuth {
		mux.Handle("/assets/{path...}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if a.handleUnauthorizedResponse(w, r, showUnauthorizedJSON) {
				return
			}

			assetsHandler.ServeHTTP(w, r)
		}))
	} else {
		mux.Handle("/assets/{path...}", assetsHandler)
	}

	server := http.Server{
		Addr:    fmt.Sprintf("%s:%d", a.Config.Server.Host, a.Config.Server.Port),
		Handler: a.securityHeadersMiddleware(mux),
	}

	start := func() error {
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

	ctx, cancelCtx := context.WithCancel(context.Background())
	go a.sseUpdateLoop(ctx)
	if a.oidcSessions != nil {
		go a.oidcSessions.runSweeper(ctx, 15*time.Minute, OIDC_SESSION_VALID_PERIOD)
	}

	stop := func() error {
		cancelCtx()
		return server.Close()
	}

	return start, stop
}
