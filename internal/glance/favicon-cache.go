package glance

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	faviconCacheDir      = "favicons"
	faviconCacheDuration = 7 * 24 * time.Hour // 1 week cache
	faviconServiceURL    = "https://www.google.com/s2/favicons?domain="
)

// getFaviconCachePath returns the path to the favicon cache directory
func (a *application) getFaviconCachePath() string {
	return filepath.Join(a.Config.Server.CachePath, faviconCacheDir)
}

// getFaviconPath returns the cached favicon path for a domain
func (a *application) getFaviconPath(domain string) string {
	// Create an MD5 hash of the domain to use as filename
	hash := md5.Sum([]byte(domain))
	filename := hex.EncodeToString(hash[:]) + ".ico"
	return filepath.Join(a.getFaviconCachePath(), filename)
}

// ensureCacheDirExists ensures the favicon cache directory exists
func (a *application) ensureCacheDirExists() error {
	cacheDir := a.getFaviconCachePath()
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return os.MkdirAll(cacheDir, 0755)
	}
	return nil
}

// handleFaviconRequest handles requests for favicons, using local cache when available
func (a *application) handleFaviconRequest(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		http.Error(w, "domain parameter is required", http.StatusBadRequest)
		return
	}

	// Ensure cache directory exists
	if err := a.ensureCacheDirExists(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	faviconPath := a.getFaviconPath(domain)
	
	    // Check if favicon exists in cache and is fresh
	    var cacheExists bool
	    if info, err := os.Stat(faviconPath); err == nil {
	        cacheExists = true
	        if time.Since(info.ModTime()) < faviconCacheDuration {
	            // Serve from cache if fresh
	            http.ServeFile(w, r, faviconPath)
	            return
	        }
	    }

	// Favicon not in cache or is expired, fetch from Google
	resp, err := http.Get(faviconServiceURL + domain)
	if err != nil || resp.StatusCode != http.StatusOK {
		// If fetch fails but we have a cached version, refresh its timestamp and use it
		if cacheExists {
			// Update modification time to extend cache lifetime
			now := time.Now()
			os.Chtimes(faviconPath, now, now)
			
			// Serve stale cached favicon
			http.ServeFile(w, r, faviconPath)
			return
		}
		
		// No cached version available, return error
		http.Error(w, "Failed to fetch favicon", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Create the cache file
	f, err := os.Create(faviconPath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Set content-type header if available
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
	    w.Header().Set("Content-Type", contentType)
	} else {
	    w.Header().Set("Content-Type", "image/x-icon")
	}

	// Set cache-control header
	w.Header().Set("Cache-Control", "public, max-age=604800") // 1 week

	// Write to cache file and response writer
	mw := io.MultiWriter(f, w)
	if _, err := io.Copy(mw, resp.Body); err != nil {
	    http.Error(w, "Internal server error", http.StatusInternalServerError)
	    return
	}
}
