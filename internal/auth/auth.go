// Package auth implements the device-code login flow against
// digger.gg/api/auth/{start,poll}.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// User mirrors the JSON the server returns.
type User struct {
	UID         string `json:"uid"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	PhotoURL    string `json:"photoURL,omitempty"`
}

// startResponse is what /api/auth/start returns.
type startResponse struct {
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	PollInterval            int    `json:"poll_interval"`
	ExpiresIn               int    `json:"expires_in"`
}

// pollResponse is what /api/auth/poll returns.
type pollResponse struct {
	Status      string `json:"status"`
	AccessToken string `json:"access_token,omitempty"`
	User        *User  `json:"user,omitempty"`
}

// Result is what Login returns when it succeeds.
type Result struct {
	Token string
	User  User
}

// Login performs the full device-code flow. It prints progress messages
// to stderr via the print callback and returns the saved token + user.
//
// baseURL should be something like "https://digger.gg".
func Login(ctx context.Context, baseURL string, print func(string)) (*Result, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if print == nil {
		print = func(string) {}
	}

	// 1. start the session
	startURL := baseURL + "/api/auth/start"
	resp, err := http.Post(startURL, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("start: HTTP %d: %s", resp.StatusCode, string(body))
	}
	var start startResponse
	if err := json.NewDecoder(resp.Body).Decode(&start); err != nil {
		return nil, fmt.Errorf("start: decode: %w", err)
	}

	// 2. show the URL + code; try to open the browser
	print("\n  open in your browser:  " + start.VerificationURIComplete)
	print("  if you can't, go to:   " + start.VerificationURI)
	print("  and enter the code:    " + start.UserCode + "\n")
	if err := openBrowser(start.VerificationURIComplete); err != nil {
		print("  (couldn't open the browser automatically — paste the URL above)")
	}
	print("  waiting for you to authorize…")

	// 3. poll
	interval := time.Duration(start.PollInterval) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	deadline := time.Now().Add(time.Duration(start.ExpiresIn) * time.Second)
	if start.ExpiresIn <= 0 {
		deadline = time.Now().Add(5 * time.Minute)
	}

	for {
		if time.Now().After(deadline) {
			return nil, errors.New("login expired — try again")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		body, _ := json.Marshal(map[string]string{"user_code": start.UserCode})
		r, err := http.Post(baseURL+"/api/auth/poll", "application/json", bytes.NewReader(body))
		if err != nil {
			continue
		}
		var pr pollResponse
		if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
			r.Body.Close()
			continue
		}
		r.Body.Close()
		switch pr.Status {
		case "pending":
			continue
		case "expired":
			return nil, errors.New("login expired — try again")
		case "approved":
			if pr.User == nil {
				return nil, errors.New("approved without user")
			}
			print("\n  ✓ authorized as " + pr.User.Email)
			return &Result{Token: pr.AccessToken, User: *pr.User}, nil
		default:
			return nil, fmt.Errorf("unexpected status %q", pr.Status)
		}
	}
}

// openBrowser opens the given URL in the platform default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
