package glance

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"
)

func TestAuthTokenGenerationAndVerification(t *testing.T) {
	secret, err := makeAuthSecretKey(AUTH_SECRET_KEY_LENGTH)
	if err != nil {
		t.Fatalf("Failed to generate secret key: %v", err)
	}

	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		t.Fatalf("Failed to decode secret key: %v", err)
	}

	if len(secretBytes) != AUTH_SECRET_KEY_LENGTH {
		t.Fatalf("Secret key length is not %d bytes", AUTH_SECRET_KEY_LENGTH)
	}

	now := time.Now()
	username := "admin"

	token, err := generateSessionToken(username, secretBytes, now)
	if err != nil {
		t.Fatalf("Failed to generate session token: %v", err)
	}

	usernameHashBytes, shouldRegen, err := verifySessionToken(token, secretBytes, now)
	if err != nil {
		t.Fatalf("Failed to verify session token: %v", err)
	}

	if shouldRegen {
		t.Fatal("Token should not need to be regenerated immediately after generation")
	}

	computedUsernameHash, err := computeUsernameHash(username, secretBytes)
	if err != nil {
		t.Fatalf("Failed to compute username hash: %v", err)
	}

	if !bytes.Equal(usernameHashBytes, computedUsernameHash) {
		t.Fatal("Username hash does not match the expected value")
	}

	// Test token regeneration
	timeRightAfterRegenPeriod := now.Add(AUTH_TOKEN_VALID_PERIOD - AUTH_TOKEN_REGEN_BEFORE + 2*time.Second)
	_, shouldRegen, err = verifySessionToken(token, secretBytes, timeRightAfterRegenPeriod)
	if err != nil {
		t.Fatalf("Token verification should not fail during regeneration period, err: %v", err)
	}

	if !shouldRegen {
		t.Fatal("Token should have been marked for regeneration")
	}

	// Test token expiration
	_, _, err = verifySessionToken(token, secretBytes, now.Add(AUTH_TOKEN_VALID_PERIOD+2*time.Second))
	if err == nil {
		t.Fatal("Expected token verification to fail after token expiration")
	}

	// Test tampered token
	decodedToken, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("Failed to decode token: %v", err)
	}

	// If any of the bytes are off by 1, the token should be considered invalid
	for i := range len(decodedToken) {
		tampered := make([]byte, len(decodedToken))
		copy(tampered, decodedToken)
		tampered[i] += 1

		_, _, err = verifySessionToken(base64.StdEncoding.EncodeToString(tampered), secretBytes, now)
		if err == nil {
			t.Fatalf("Expected token verification to fail for tampered token at index %d", i)
		}
	}
}
