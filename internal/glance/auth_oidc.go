package glance

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	OIDC_STATE_COOKIE_NAME   = "oidc_state"
	OIDC_STATE_VALID_PERIOD  = 10 * time.Minute
	OIDC_STATE_MAX_DATA_SIZE = 2048
)

var defaultOIDCScopes = []string{oidc.ScopeOpenID, "profile", "email"}

type oidcFlowState struct {
	State        string `json:"state"`
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"code_verifier"`
	RedirectURL  string `json:"redirect_url"`
}

func (a *application) initOIDCAuth() error {
	oidcConfig := a.Config.Auth.OIDC
	if oidcConfig.Issuer == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, oidcConfig.Issuer)
	if err != nil {
		return fmt.Errorf("initializing oidc provider: %w", err)
	}

	scopes := oidcConfig.Scopes
	if len(scopes) == 0 {
		scopes = defaultOIDCScopes
	}

	a.oidcVerifier = provider.Verifier(&oidc.Config{ClientID: oidcConfig.ClientID})
	a.oauth2Config = &oauth2.Config{
		ClientID:     oidcConfig.ClientID,
		ClientSecret: oidcConfig.ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}
	a.oidcUsernames = make(map[string]struct{})
	a.oidcEnabled = true

	return nil
}

func (a *application) OIDCAuthEnabled() bool {
	return a.oidcEnabled
}

func (a *application) LocalAuthEnabled() bool {
	return len(a.Config.Auth.Users) > 0
}

func (a *application) showLocalLoginForm() bool {
	return a.LocalAuthEnabled() && !a.Config.Auth.OIDC.DisableLocalLogin
}

func (a *application) oidcRedirectURL(r *http.Request) string {
	if a.Config.Auth.OIDC.RedirectURL != "" {
		return a.Config.Auth.OIDC.RedirectURL
	}

	scheme := "http"
	if strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https" {
		scheme = "https"
	}

	return scheme + "://" + r.Host + a.Config.Server.BaseURL + "/auth/oidc/callback"
}

func (a *application) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !a.oidcEnabled {
		http.NotFound(w, r)
		return
	}

	state, err := randomOIDCString(32)
	if err != nil {
		log.Printf("Could not generate oidc state: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	nonce, err := randomOIDCString(32)
	if err != nil {
		log.Printf("Could not generate oidc nonce: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	codeVerifier := oauth2.GenerateVerifier()
	redirectURL := a.oidcRedirectURL(r)

	flowState := oidcFlowState{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		RedirectURL:  redirectURL,
	}

	if err := a.setOIDCStateCookie(w, r, flowState); err != nil {
		log.Printf("Could not set oidc state cookie: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	oauth2Config := *a.oauth2Config
	oauth2Config.RedirectURL = redirectURL

	authURL := oauth2Config.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(codeVerifier),
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (a *application) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !a.oidcEnabled {
		http.NotFound(w, r)
		return
	}

	ip := a.addressOfRequest(r)

	if a.recordOIDCAuthFailure(ip) {
		a.redirectOIDCLoginError(w, r)
		return
	}

	flowState, err := a.readOIDCStateCookie(r)
	if err != nil {
		log.Printf("Invalid oidc state cookie from %s: %v", ip, err)
		a.clearOIDCStateCookie(w, r)
		a.redirectOIDCLoginError(w, r)
		return
	}

	a.clearOIDCStateCookie(w, r)

	if r.URL.Query().Get("state") != flowState.State {
		log.Printf("Mismatched oidc state from %s", ip)
		a.redirectOIDCLoginError(w, r)
		return
	}

	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		log.Printf("OIDC provider returned error for %s: %s (%s)", ip, errMsg, r.URL.Query().Get("error_description"))
		a.redirectOIDCLoginError(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		log.Printf("Missing oidc authorization code from %s", ip)
		a.redirectOIDCLoginError(w, r)
		return
	}

	oauth2Config := *a.oauth2Config
	oauth2Config.RedirectURL = flowState.RedirectURL

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	token, err := oauth2Config.Exchange(
		ctx,
		code,
		oauth2.VerifierOption(flowState.CodeVerifier),
	)
	if err != nil {
		log.Printf("Could not exchange oidc code from %s: %v", ip, err)
		a.redirectOIDCLoginError(w, r)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		log.Printf("Missing id_token in oidc response for %s", ip)
		a.redirectOIDCLoginError(w, r)
		return
	}

	idToken, err := a.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Printf("Could not verify oidc id token from %s: %v", ip, err)
		a.redirectOIDCLoginError(w, r)
		return
	}

	if idToken.Nonce != flowState.Nonce {
		log.Printf("Mismatched oidc nonce from %s", ip)
		a.redirectOIDCLoginError(w, r)
		return
	}

	username, err := usernameFromOIDCToken(idToken, a.Config.Auth.OIDC.UsernameClaim)
	if err != nil {
		log.Printf("Could not resolve oidc username from %s: %v", ip, err)
		a.redirectOIDCLoginError(w, r)
		return
	}

	if err := a.registerOIDCUser(username); err != nil {
		log.Printf("Could not register oidc user %q from %s: %v", username, ip, err)
		a.redirectOIDCLoginError(w, r)
		return
	}

	sessionToken, err := generateSessionToken(username, a.authSecretKey, time.Now())
	if err != nil {
		log.Printf("Could not compute session token for oidc user %q: %v", username, err)
		a.redirectOIDCLoginError(w, r)
		return
	}

	a.setAuthSessionCookie(w, r, sessionToken, time.Now().Add(AUTH_TOKEN_VALID_PERIOD))

	a.authAttemptsMu.Lock()
	delete(a.failedAuthAttempts, ip)
	a.authAttemptsMu.Unlock()

	http.Redirect(w, r, a.Config.Server.BaseURL+"/", http.StatusSeeOther)
}

func (a *application) registerOIDCUser(username string) error {
	usernameHash, err := computeUsernameHash(username, a.authSecretKey)
	if err != nil {
		return err
	}

	a.oidcUsernamesMu.Lock()
	defer a.oidcUsernamesMu.Unlock()

	a.usernameHashToUsername[string(usernameHash)] = username
	a.oidcUsernames[username] = struct{}{}

	return nil
}

func (a *application) isOIDCUser(username string) bool {
	a.oidcUsernamesMu.RLock()
	defer a.oidcUsernamesMu.RUnlock()

	_, ok := a.oidcUsernames[username]
	return ok
}

func usernameFromOIDCToken(idToken *oidc.IDToken, usernameClaim string) (string, error) {
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return "", err
	}

	if usernameClaim != "" {
		username, ok := stringClaim(claims, usernameClaim)
		if !ok {
			return "", fmt.Errorf("username claim %q is missing or empty", usernameClaim)
		}
		return username, nil
	}

	for _, claim := range []string{"preferred_username", "email", "sub"} {
		if username, ok := stringClaim(claims, claim); ok {
			return username, nil
		}
	}

	return "", fmt.Errorf("no usable username claim found in id token")
}

func stringClaim(claims map[string]any, name string) (string, bool) {
	value, ok := claims[name]
	if !ok {
		return "", false
	}

	claim, ok := value.(string)
	if !ok || claim == "" {
		return "", false
	}

	return claim, true
}

func (a *application) recordOIDCAuthFailure(ip string) bool {
	a.authAttemptsMu.Lock()
	defer a.authAttemptsMu.Unlock()

	attempt, exists := a.failedAuthAttempts[ip]
	if !exists {
		a.failedAuthAttempts[ip] = &failedAuthAttempt{
			attempts: 1,
			first:    time.Now(),
		}
		return false
	}

	elapsed := time.Since(attempt.first)
	if elapsed >= AUTH_RATE_LIMIT_WINDOW {
		a.failedAuthAttempts[ip] = &failedAuthAttempt{
			attempts: 1,
			first:    time.Now(),
		}
		return false
	}

	attempt.attempts++
	return attempt.attempts > AUTH_RATE_LIMIT_MAX_ATTEMPTS
}

func (a *application) redirectOIDCLoginError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.Config.Server.BaseURL+"/login?error=oidc", http.StatusSeeOther)
}

func randomOIDCString(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func (a *application) setOIDCStateCookie(w http.ResponseWriter, r *http.Request, flowState oidcFlowState) error {
	payload, err := json.Marshal(flowState)
	if err != nil {
		return err
	}

	if len(payload) > OIDC_STATE_MAX_DATA_SIZE {
		return fmt.Errorf("oidc state payload is too large")
	}

	expires := time.Now().Add(OIDC_STATE_VALID_PERIOD)
	data := make([]byte, AUTH_TIMESTAMP_LENGTH+len(payload))
	binary.LittleEndian.PutUint32(data[0:AUTH_TIMESTAMP_LENGTH], uint32(expires.Unix()))
	copy(data[AUTH_TIMESTAMP_LENGTH:], payload)

	h := hmac.New(sha256.New, a.authSecretKey[0:AUTH_TOKEN_SECRET_LENGTH])
	h.Write(data)
	signed := base64.StdEncoding.EncodeToString(append(data, h.Sum(nil)...))

	http.SetCookie(w, &http.Cookie{
		Name:     OIDC_STATE_COOKIE_NAME,
		Value:    signed,
		Expires:  expires,
		Secure:   strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https",
		Path:     a.Config.Server.BaseURL + "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
	})

	return nil
}

func (a *application) readOIDCStateCookie(r *http.Request) (oidcFlowState, error) {
	var flowState oidcFlowState

	cookie, err := r.Cookie(OIDC_STATE_COOKIE_NAME)
	if err != nil || cookie.Value == "" {
		return flowState, fmt.Errorf("oidc state cookie is missing")
	}

	tokenBytes, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return flowState, err
	}

	if len(tokenBytes) < AUTH_TIMESTAMP_LENGTH+32 {
		return flowState, fmt.Errorf("oidc state cookie is too short")
	}

	dataEnd := len(tokenBytes) - 32
	data := tokenBytes[0:dataEnd]
	providedSignature := tokenBytes[dataEnd:]

	h := hmac.New(sha256.New, a.authSecretKey[0:AUTH_TOKEN_SECRET_LENGTH])
	h.Write(data)
	if !hmac.Equal(h.Sum(nil), providedSignature) {
		return flowState, fmt.Errorf("oidc state cookie signature is invalid")
	}

	expires := int64(binary.LittleEndian.Uint32(data[0:AUTH_TIMESTAMP_LENGTH]))
	if time.Now().Unix() > expires {
		return flowState, fmt.Errorf("oidc state cookie has expired")
	}

	payload := data[AUTH_TIMESTAMP_LENGTH:]
	if len(payload) > OIDC_STATE_MAX_DATA_SIZE {
		return flowState, fmt.Errorf("oidc state payload is too large")
	}

	if err := json.Unmarshal(payload, &flowState); err != nil {
		return flowState, err
	}

	if flowState.State == "" || flowState.Nonce == "" || flowState.CodeVerifier == "" || flowState.RedirectURL == "" {
		return flowState, fmt.Errorf("oidc state cookie is incomplete")
	}

	return flowState, nil
}

func (a *application) clearOIDCStateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     OIDC_STATE_COOKIE_NAME,
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		Secure:   strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https",
		Path:     a.Config.Server.BaseURL + "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
	})
}
