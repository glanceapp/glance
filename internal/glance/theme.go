package glance

import (
	"fmt"
	"html/template"
	"net/http"
	"time"
)

var (
	themeStyleTemplate         = mustParseTemplate("theme-style.gotmpl")
	themePresetPreviewTemplate = mustParseTemplate("theme-preset-preview.html")
)

func (a *application) handleThemeChangeRequest(w http.ResponseWriter, r *http.Request) {
	themeKey := r.PathValue("key")

	properties, exists := a.Config.Theme.Presets.Get(themeKey)
	if !exists && themeKey != "default" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if themeKey == "default" {
		properties = &a.Config.Theme.themeProperties
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "theme",
		Value:    themeKey,
		Path:     a.Config.Server.BaseURL + "/",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(2 * 365 * 24 * time.Hour),
	})

	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("X-Scheme", ternary(properties.Light, "light", "dark"))
	w.Write([]byte(properties.CSS))
}

type themeProperties struct {
	BackgroundColor          *hslColorField `yaml:"background-color"`
	PrimaryColor             *hslColorField `yaml:"primary-color"`
	PositiveColor            *hslColorField `yaml:"positive-color"`
	NegativeColor            *hslColorField `yaml:"negative-color"`
	Light                    bool           `yaml:"light"`
	ContrastMultiplier       float32        `yaml:"contrast-multiplier"`
	TextSaturationMultiplier float32        `yaml:"text-saturation-multiplier"`

	Key                  string        `yaml:"-"`
	CSS                  template.CSS  `yaml:"-"`
	PreviewHTML          template.HTML `yaml:"-"`
	BackgroundColorAsHex string        `yaml:"-"`
}

func (t *themeProperties) init() error {
	css, err := executeTemplateToString(themeStyleTemplate, t)
	if err != nil {
		return fmt.Errorf("compiling theme style: %v", err)
	}
	t.CSS = template.CSS(whitespaceAtBeginningOfLinePattern.ReplaceAllString(css, ""))

	previewHTML, err := executeTemplateToString(themePresetPreviewTemplate, t)
	if err != nil {
		return fmt.Errorf("compiling theme preview: %v", err)
	}
	t.PreviewHTML = template.HTML(previewHTML)

	if t.BackgroundColor != nil {
		t.BackgroundColorAsHex = t.BackgroundColor.ToHex()
	} else {
		t.BackgroundColorAsHex = "#151519"
	}

	return nil
}

func (t1 *themeProperties) SameAs(t2 *themeProperties) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	if t1.Light != t2.Light {
		return false
	}
	if t1.ContrastMultiplier != t2.ContrastMultiplier {
		return false
	}
	if t1.TextSaturationMultiplier != t2.TextSaturationMultiplier {
		return false
	}
	if !t1.BackgroundColor.SameAs(t2.BackgroundColor) {
		return false
	}
	if !t1.PrimaryColor.SameAs(t2.PrimaryColor) {
		return false
	}
	if !t1.PositiveColor.SameAs(t2.PositiveColor) {
		return false
	}
	if !t1.NegativeColor.SameAs(t2.NegativeColor) {
		return false
	}
	return true
}
