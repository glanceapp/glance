package glance

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOIDCStateCookieRoundTrip(t *testing.T) {
	secret, err := makeAuthSecretKey(AUTH_SECRET_KEY_LENGTH)
	if err != nil {
		t.Fatalf("Failed to generate secret key: %v", err)
	}

	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		t.Fatalf("Failed to decode secret key: %v", err)
	}

	app := &application{
		authSecretKey: secretBytes,
		Config: config{
			Server: struct {
				Host       string `yaml:"host"`
				Port       uint16 `yaml:"port"`
				Proxied    bool   `yaml:"proxied"`
				AssetsPath string `yaml:"assets-path"`
				BaseURL    string `yaml:"base-url"`
			}{
				BaseURL: "",
			},
		},
	}

	flowState := oidcFlowState{
		State:        "state-value",
		Nonce:        "nonce-value",
		CodeVerifier: "verifier-value",
		RedirectURL:  "https://glance.example.com/auth/oidc/callback",
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()

	if err := app.setOIDCStateCookie(recorder, request, flowState); err != nil {
		t.Fatalf("Failed to set oidc state cookie: %v", err)
	}

	response := recorder.Result()
	if len(response.Cookies()) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(response.Cookies()))
	}

	callbackRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	callbackRequest.AddCookie(response.Cookies()[0])

	readState, err := app.readOIDCStateCookie(callbackRequest)
	if err != nil {
		t.Fatalf("Failed to read oidc state cookie: %v", err)
	}

	if readState != flowState {
		t.Fatalf("Expected %+v, got %+v", flowState, readState)
	}
}

func TestUsernameFromOIDCClaims(t *testing.T) {
	tests := []struct {
		name          string
		claims        map[string]any
		usernameClaim string
		want          string
		wantErr       bool
	}{
		{
			name:          "explicit claim",
			claims:        map[string]any{"preferred_username": "alice"},
			usernameClaim: "preferred_username",
			want:          "alice",
		},
		{
			name:    "fallback to email",
			claims:  map[string]any{"email": "alice@example.com"},
			want:    "alice@example.com",
		},
		{
			name:    "fallback to sub",
			claims:  map[string]any{"sub": "provider-user-id"},
			want:    "provider-user-id",
		},
		{
			name:          "missing explicit claim",
			claims:        map[string]any{"email": "alice@example.com"},
			usernameClaim: "preferred_username",
			wantErr:       true,
		},
		{
			name:    "no usable claims",
			claims:  map[string]any{},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			username, err := usernameFromClaimsMap(test.claims, test.usernameClaim)
			if test.wantErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if username != test.want {
				t.Fatalf("Expected username %q, got %q", test.want, username)
			}
		})
	}
}

func TestRegisterOIDCUserAllowsAuthorization(t *testing.T) {
	secret, err := makeAuthSecretKey(AUTH_SECRET_KEY_LENGTH)
	if err != nil {
		t.Fatalf("Failed to generate secret key: %v", err)
	}

	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		t.Fatalf("Failed to decode secret key: %v", err)
	}

	app := &application{
		RequiresAuth:           true,
		authSecretKey:          secretBytes,
		usernameHashToUsername: make(map[string]string),
		oidcUsernames:          make(map[string]struct{}),
		oidcEnabled:            true,
		Config: config{
			Auth: struct {
				SecretKey string           `yaml:"secret-key"`
				Users     map[string]*user `yaml:"users"`
				OIDC      oidcConfig       `yaml:"oidc"`
			}{
				Users: map[string]*user{},
			},
		},
	}

	if err := app.registerOIDCUser("oidc-user"); err != nil {
		t.Fatalf("Failed to register oidc user: %v", err)
	}

	token, err := generateSessionToken("oidc-user", secretBytes, time.Now())
	if err != nil {
		t.Fatalf("Failed to generate session token: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: AUTH_SESSION_COOKIE_NAME, Value: token})
	recorder := httptest.NewRecorder()

	if !app.isAuthorized(recorder, request) {
		t.Fatal("Expected oidc user to be authorized")
	}
}

func usernameFromClaimsMap(claims map[string]any, usernameClaim string) (string, error) {
	if usernameClaim != "" {
		username, ok := stringClaim(claims, usernameClaim)
		if !ok {
			return "", errMissingClaim
		}
		return username, nil
	}

	for _, claim := range []string{"preferred_username", "email", "sub"} {
		if username, ok := stringClaim(claims, claim); ok {
			return username, nil
		}
	}

	return "", errMissingClaim
}

var errMissingClaim = errTestMissingClaim{}

type errTestMissingClaim struct{}

func (errTestMissingClaim) Error() string {
	return "missing claim"
}

func TestOIDCRedirectURLUsesRequestHost(t *testing.T) {
	app := &application{
		Config: config{
			Server: struct {
				Host       string `yaml:"host"`
				Port       uint16 `yaml:"port"`
				Proxied    bool   `yaml:"proxied"`
				AssetsPath string `yaml:"assets-path"`
				BaseURL    string `yaml:"base-url"`
			}{
				BaseURL: "/glance",
			},
		},
	}

	request := httptest.NewRequest(http.MethodGet, "http://example.com/glance/login", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Host = "example.com"

	got := app.oidcRedirectURL(request)
	want := "https://example.com/glance/auth/oidc/callback"
	if got != want {
		t.Fatalf("Expected %q, got %q", want, got)
	}
}
