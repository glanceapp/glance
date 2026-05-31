package glance

import (
	"bytes"
	"strings"
	"testing"
)

func loginTemplateData(app *application) *templateData {
	return &templateData{
		App:            app,
		ShowLocalLogin: app.showLocalLoginForm(),
		ShowOIDCLogin:  app.oidcEnabled,
		Request: templateRequestData{
			Theme: &app.Config.Theme.themeProperties,
		},
	}
}

func TestLoginTemplateLocalAndOIDC(t *testing.T) {
	app := &application{
		RequiresAuth: true,
		oidcEnabled:  true,
		Config: config{
			Auth: struct {
				SecretKey string           `yaml:"secret-key"`
				Users     map[string]*user `yaml:"users"`
				OIDC      oidcConfig       `yaml:"oidc"`
			}{
				Users: map[string]*user{"admin": {PasswordHash: []byte("x")}},
				OIDC: oidcConfig{
					Issuer:            "https://example.com",
					DisableLocalLogin: false,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := loginPageTemplate.Execute(&buf, loginTemplateData(app)); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, `id="username"`) {
		t.Fatalf("expected username field, got:\n%s", out)
	}
	if !strings.Contains(out, "SIGN IN WITH SSO") {
		t.Fatal("expected SSO button")
	}
}

func TestLoginTemplateOIDCOnly(t *testing.T) {
	app := &application{
		RequiresAuth: true,
		oidcEnabled:  true,
		Config: config{
			Auth: struct {
				SecretKey string           `yaml:"secret-key"`
				Users     map[string]*user `yaml:"users"`
				OIDC      oidcConfig       `yaml:"oidc"`
			}{
				Users: map[string]*user{},
				OIDC: oidcConfig{
					Issuer:            "https://example.com",
					DisableLocalLogin: false,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := loginPageTemplate.Execute(&buf, loginTemplateData(app)); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if strings.Contains(out, `id="username"`) {
		t.Fatalf("should not show username without local users")
	}
	if !strings.Contains(out, "SIGN IN WITH SSO") {
		t.Fatal("expected SSO button")
	}
}

func TestLoginTemplateDisableLocalLogin(t *testing.T) {
	app := &application{
		RequiresAuth: true,
		oidcEnabled:  true,
		Config: config{
			Auth: struct {
				SecretKey string           `yaml:"secret-key"`
				Users     map[string]*user `yaml:"users"`
				OIDC      oidcConfig       `yaml:"oidc"`
			}{
				Users: map[string]*user{"admin": {PasswordHash: []byte("x")}},
				OIDC: oidcConfig{
					Issuer:            "https://example.com",
					DisableLocalLogin: true,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := loginPageTemplate.Execute(&buf, loginTemplateData(app)); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if strings.Contains(out, `id="username"`) {
		t.Fatalf("should not show username when disable-local-login is true")
	}
}
