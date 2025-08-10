package glance

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathrand "math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const AUTH_SESSION_COOKIE_NAME = "session_token"
const AUTH_RATE_LIMIT_WINDOW = 5 * time.Minute
const AUTH_RATE_LIMIT_MAX_ATTEMPTS = 5

const AUTH_TOKEN_SECRET_LENGTH = 32
const AUTH_USERNAME_HASH_LENGTH = 32
const AUTH_SECRET_KEY_LENGTH = AUTH_TOKEN_SECRET_LENGTH + AUTH_USERNAME_HASH_LENGTH
const AUTH_TIMESTAMP_LENGTH = 4 // uint32
const AUTH_TOKEN_DATA_LENGTH = AUTH_USERNAME_HASH_LENGTH + AUTH_TIMESTAMP_LENGTH

// How long the token will be valid for
const AUTH_TOKEN_VALID_PERIOD = 14 * 24 * time.Hour // 14 days
// How long the token has left before it should be regenerated
const AUTH_TOKEN_REGEN_BEFORE = 7 * 24 * time.Hour // 7 days

var loginPageTemplate = mustParseTemplate("login.html", "document.html", "footer.html")

type doWhenUnauthorized int

const (
	redirectToLogin doWhenUnauthorized = iota
	showUnauthorizedJSON
)

type failedAuthAttempt struct {
	attempts int
	first    time.Time
}

func generateSessionToken(username string, secret []byte, now time.Time) (string, error) {
	if len(secret) != AUTH_SECRET_KEY_LENGTH {
		return "", fmt.Errorf("secret key length is not %d bytes", AUTH_SECRET_KEY_LENGTH)
	}

	usernameHash, err := computeUsernameHash(username, secret)
	if err != nil {
		return "", err
	}

	data := make([]byte, AUTH_TOKEN_DATA_LENGTH)
	copy(data, usernameHash)
	expires := now.Add(AUTH_TOKEN_VALID_PERIOD).Unix()
	binary.LittleEndian.PutUint32(data[AUTH_USERNAME_HASH_LENGTH:], uint32(expires))

	h := hmac.New(sha256.New, secret[0:AUTH_TOKEN_SECRET_LENGTH])
	h.Write(data)

	signature := h.Sum(nil)
	encodedToken := base64.StdEncoding.EncodeToString(append(data, signature...))
	// encodedToken ends up being (hashed username + expiration timestamp + signature) encoded as base64

	return encodedToken, nil
}

func computeUsernameHash(username string, secret []byte) ([]byte, error) {
	if len(secret) != AUTH_SECRET_KEY_LENGTH {
		return nil, fmt.Errorf("secret key length is not %d bytes", AUTH_SECRET_KEY_LENGTH)
	}

	h := hmac.New(sha256.New, secret[AUTH_TOKEN_SECRET_LENGTH:])
	h.Write([]byte(username))

	return h.Sum(nil), nil
}

func verifySessionToken(token string, secretBytes []byte, now time.Time) ([]byte, bool, error) {
	tokenBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, false, err
	}

	if len(tokenBytes) != AUTH_TOKEN_DATA_LENGTH+32 {
		return nil, false, fmt.Errorf("token length is invalid")
	}

	if len(secretBytes) != AUTH_SECRET_KEY_LENGTH {
		return nil, false, fmt.Errorf("secret key length is not %d bytes", AUTH_SECRET_KEY_LENGTH)
	}

	usernameHashBytes := tokenBytes[0:AUTH_USERNAME_HASH_LENGTH]
	timestampBytes := tokenBytes[AUTH_USERNAME_HASH_LENGTH : AUTH_USERNAME_HASH_LENGTH+AUTH_TIMESTAMP_LENGTH]
	providedSignatureBytes := tokenBytes[AUTH_TOKEN_DATA_LENGTH:]

	h := hmac.New(sha256.New, secretBytes[0:32])
	h.Write(tokenBytes[0:AUTH_TOKEN_DATA_LENGTH])
	expectedSignatureBytes := h.Sum(nil)

	if !hmac.Equal(expectedSignatureBytes, providedSignatureBytes) {
		return nil, false, fmt.Errorf("signature does not match")
	}

	expiresTimestamp := int64(binary.LittleEndian.Uint32(timestampBytes))
	if now.Unix() > expiresTimestamp {
		return nil, false, fmt.Errorf("token has expired")
	}

	return usernameHashBytes,
		// True if the token should be regenerated
		time.Unix(expiresTimestamp, 0).Add(-AUTH_TOKEN_REGEN_BEFORE).Before(now),
		nil
}

func makeAuthSecretKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func (a *application) handleAuthenticationAttempt(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	waitOnFailure := 1*time.Second - time.Duration(mathrand.IntN(500))*time.Millisecond

	ip := a.addressOfRequest(r)

	a.authAttemptsMu.Lock()
	exceededRateLimit, retryAfter := func() (bool, int) {
		attempt, exists := a.failedAuthAttempts[ip]
		if !exists {
			a.failedAuthAttempts[ip] = &failedAuthAttempt{
				attempts: 1,
				first:    time.Now(),
			}

			return false, 0
		}

		elapsed := time.Since(attempt.first)
		if elapsed < AUTH_RATE_LIMIT_WINDOW && attempt.attempts >= AUTH_RATE_LIMIT_MAX_ATTEMPTS {
			return true, max(1, int(AUTH_RATE_LIMIT_WINDOW.Seconds()-elapsed.Seconds()))
		}

		attempt.attempts++
		return false, 0
	}()

	if exceededRateLimit {
		a.authAttemptsMu.Unlock()
		time.Sleep(waitOnFailure)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		w.WriteHeader(http.StatusTooManyRequests)
		return
	} else {
		// Clean up old failed attempts
		for ipOfAttempt := range a.failedAuthAttempts {
			if time.Since(a.failedAuthAttempts[ipOfAttempt].first) > AUTH_RATE_LIMIT_WINDOW {
				delete(a.failedAuthAttempts, ipOfAttempt)
			}
		}
		a.authAttemptsMu.Unlock()
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	err = json.Unmarshal(body, &creds)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logAuthFailure := func() {
		log.Printf(
			"Failed login attempt for user '%s' from %s",
			creds.Username, ip,
		)
	}

	if len(creds.Username) == 0 || len(creds.Password) == 0 {
		time.Sleep(waitOnFailure)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if len(creds.Username) > 50 || len(creds.Password) > 100 {
		logAuthFailure()
		time.Sleep(waitOnFailure)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	u, exists := a.Config.Auth.Users[creds.Username]
	if !exists {
		logAuthFailure()
		time.Sleep(waitOnFailure)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(creds.Password)); err != nil {
		logAuthFailure()
		time.Sleep(waitOnFailure)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, err := generateSessionToken(creds.Username, a.authSecretKey, time.Now())
	if err != nil {
		log.Printf("Could not compute session token during login attempt: %v", err)
		time.Sleep(waitOnFailure)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	a.setAuthSessionCookie(w, r, token, time.Now().Add(AUTH_TOKEN_VALID_PERIOD))

	a.authAttemptsMu.Lock()
	delete(a.failedAuthAttempts, ip)
	a.authAttemptsMu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (a *application) isAuthorized(w http.ResponseWriter, r *http.Request) bool {
	if !a.RequiresAuth {
		return true
	}

	token, err := r.Cookie(AUTH_SESSION_COOKIE_NAME)
	if err != nil || token.Value == "" {
		return false
	}

	usernameHash, shouldRegenerate, err := verifySessionToken(token.Value, a.authSecretKey, time.Now())
	if err != nil {
		return false
	}

	username, exists := a.usernameHashToUsername[string(usernameHash)]
	if !exists {
		return false
	}

	_, exists = a.Config.Auth.Users[username]
	if !exists {
		return false
	}

	if shouldRegenerate {
		newToken, err := generateSessionToken(username, a.authSecretKey, time.Now())
		if err != nil {
			log.Printf("Could not compute session token during regeneration: %v", err)
			return false
		}

		a.setAuthSessionCookie(w, r, newToken, time.Now().Add(AUTH_TOKEN_VALID_PERIOD))
	}

	return true
}

// Handles sending the appropriate response for an unauthorized request and returns true if the request was unauthorized
func (a *application) handleUnauthorizedResponse(w http.ResponseWriter, r *http.Request, fallback doWhenUnauthorized) bool {
	if a.isAuthorized(w, r) {
		return false
	}

	switch fallback {
	case redirectToLogin:
		http.Redirect(w, r, a.Config.Server.BaseURL+"/login", http.StatusSeeOther)
	case showUnauthorizedJSON:
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Unauthorized"}`))
	}

	return true
}

// Maybe this should be a POST request instead?
func (a *application) handleLogoutRequest(w http.ResponseWriter, r *http.Request) {
	a.setAuthSessionCookie(w, r, "", time.Now().Add(-1*time.Hour))
	http.Redirect(w, r, a.Config.Server.BaseURL+"/login", http.StatusSeeOther)
}

func (a *application) setAuthSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     AUTH_SESSION_COOKIE_NAME,
		Value:    token,
		Expires:  expires,
		Secure:   strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https",
		Path:     a.Config.Server.BaseURL + "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
	})
}

func (a *application) handleLoginPageRequest(w http.ResponseWriter, r *http.Request) {
	if a.isAuthorized(w, r) {
		http.Redirect(w, r, a.Config.Server.BaseURL+"/", http.StatusSeeOther)
		return
	}

	data := &templateData{
		App: a,
	}
	a.populateTemplateRequestData(&data.Request, r)

	var responseBytes bytes.Buffer
	err := loginPageTemplate.Execute(&responseBytes, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Write(responseBytes.Bytes())
}
