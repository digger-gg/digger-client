package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Patch openBrowser so tests don't try to launch chromium under
// CI / a headless box. Side-effect: we just record the URL it would open.
var openedURL atomic.Value

func init() {
	openBrowserOverride = func(u string) error {
		openedURL.Store(u)
		return nil
	}
}

// Login ignores browser failures, so a passing-through stub is fine for
// happy-path tests.

func TestLogin_HappyPath(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/start":
			json.NewEncoder(w).Encode(map[string]any{
				"user_code":                 "TEST-1234",
				"verification_uri":          "https://example/auth",
				"verification_uri_complete": "https://example/auth/TEST-1234",
				"poll_interval":             1, // we'll override via context
				"expires_in":                30,
			})
		case "/api/auth/poll":
			pollCount++
			if pollCount < 2 {
				json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"status":       "approved",
				"access_token": "tok-123",
				"user": map[string]any{
					"uid":         "u1",
					"email":       "ando@example.com",
					"displayName": "Ando",
					"photoURL":    "",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	openedURL.Store("")
	res, err := Login(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Token != "tok-123" {
		t.Errorf("token: got %q, want %q", res.Token, "tok-123")
	}
	if res.User.UID != "u1" || res.User.Email != "ando@example.com" {
		t.Errorf("user: %+v", res.User)
	}
	if u, _ := openedURL.Load().(string); !strings.Contains(u, "/auth/TEST-1234") {
		t.Errorf("expected browser opened at /auth/TEST-1234, got %q", u)
	}
}

func TestLogin_Expired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/start":
			json.NewEncoder(w).Encode(map[string]any{
				"user_code":                 "GONE-9999",
				"verification_uri":          "https://example/auth",
				"verification_uri_complete": "https://example/auth/GONE-9999",
				"poll_interval":             1,
				"expires_in":                30,
			})
		case "/api/auth/poll":
			json.NewEncoder(w).Encode(map[string]any{"status": "expired"})
		}
	}))
	defer srv.Close()

	_, err := Login(context.Background(), srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got %v", err)
	}
}

func TestLogin_StartFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()
	_, err := Login(context.Background(), srv.URL, nil)
	if err == nil {
		t.Error("expected error when start returns 500")
	}
}

func TestLogin_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/start":
			json.NewEncoder(w).Encode(map[string]any{
				"user_code":                 "WAIT-0000",
				"verification_uri":          "https://example/auth",
				"verification_uri_complete": "https://example/auth/WAIT-0000",
				"poll_interval":             1,
				"expires_in":                30,
			})
		case "/api/auth/poll":
			json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_, err := Login(ctx, srv.URL, nil)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}
