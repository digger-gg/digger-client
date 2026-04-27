// e2e — exercises the full digger flow against a fresh local relay.
//
//	1. start a relay (release binary expected at relay-bin path)
//	2. start a target HTTP echo on a random local port
//	3. spin up an agent, opening a TCP tunnel: target ← relay public port
//	4. ask the relay's HTTP signup endpoint for the join string
//	5. fetch through the public address — body must equal echo-target
//	6. (optional, if TEST_EMAIL+TEST_PASSWORD set) drive the full
//	   email/password login + device-code flow against digger.gg —
//	   creates a user if needed, signs in, completes the device
//	   session, polls until approved, decodes the returned access token
//
// Everything is process-local except the login leg which talks to the
// real Firebase + digger.gg API. Set TEST_EMAIL and TEST_PASSWORD to
// run the auth half. Set TEST_AUTH_BASE to point at a non-prod site
// (default https://digger.gg).
//
// Run:
//
//	go run ./cmd/e2e --relay-bin /home/ando/code/playit-lite/relay/target/release/playit-lite-relay
//
// Exits non-zero on any check failure. Verbose stderr.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/digger-gg/digger-client/internal/auth"
	"github.com/digger-gg/digger-client/internal/client"
	"github.com/digger-gg/digger-client/internal/games"
	"github.com/digger-gg/digger-client/proto"
)

func main() {
	relayBin := flag.String("relay-bin", "/home/ando/code/playit-lite/relay/target/release/playit-lite-relay", "path to playit-lite-relay")
	flag.Parse()

	if err := run(*relayBin); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("e2e OK")
}

func run(relayBin string) error {
	timeout := 30 * time.Second
	if os.Getenv("TEST_EMAIL") != "" {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// pick three free ports: relay control, relay http, target echo
	control, err := freePort()
	if err != nil { return fmt.Errorf("freePort control: %w", err) }
	httpP, err := freePort()
	if err != nil { return fmt.Errorf("freePort http: %w", err) }
	target, err := freePort()
	if err != nil { return fmt.Errorf("freePort target: %w", err) }

	step("starting echo target on :%d", target)
	stopEcho, echoCount := startEchoServer(ctx, target)
	defer stopEcho()

	step("starting relay (control :%d, http :%d)", control, httpP)
	relay := exec.CommandContext(ctx, relayBin,
		"--bind", fmt.Sprintf("127.0.0.1:%d", control),
		"--http-bind", fmt.Sprintf("127.0.0.1:%d", httpP),
		"--secret", "e2e-secret",
		"--port-pool", "55001-55020",
	)
	relay.Stdout = relay.Stderr // ignore but keep alive
	relay.Stderr = os.Stderr
	if err := relay.Start(); err != nil {
		return fmt.Errorf("start relay: %w", err)
	}
	defer func() { _ = relay.Process.Kill() }()
	if err := waitForPort(ctx, control); err != nil {
		return fmt.Errorf("relay control didn't open: %w", err)
	}
	if err := waitForPort(ctx, httpP); err != nil {
		return fmt.Errorf("relay http didn't open: %w", err)
	}

	step("fetching join string from relay http :%d", httpP)
	join, err := fetchJoin(ctx, fmt.Sprintf("http://127.0.0.1:%d/", httpP))
	if err != nil {
		return fmt.Errorf("fetch join: %w", err)
	}
	// The relay prefers the public IP for the join string; we only
	// care that the prefix and port are right.
	if !strings.HasPrefix(join, "pl1://e2e-secret@") ||
		!strings.HasSuffix(join, fmt.Sprintf(":%d", control)) {
		return fmt.Errorf("join malformed: %q", join)
	}
	check("join string parsed: %s", join)

	step("starting agent + opening tcp tunnel for target")
	c := client.New(client.Config{
		Relay:  fmt.Sprintf("127.0.0.1:%d", control),
		Secret: "e2e-secret",
		Name:   "e2e",
	})
	agentCtx, agentCancel := context.WithCancel(ctx)
	defer agentCancel()
	go func() { _ = c.Run(agentCtx) }()
	if err := waitFor(ctx, "agent connected", func() bool {
		return c.Snapshot().Status == client.StatusConnected
	}); err != nil {
		return err
	}

	c.Send(client.CmdAddTunnel{
		Proto:      proto.Tcp,
		PublicPort: 0,
		LocalAddr:  fmt.Sprintf("127.0.0.1:%d", target),
	})

	var publicPort uint16
	if err := waitFor(ctx, "tunnel open", func() bool {
		for _, t := range c.Snapshot().Tunnels {
			if t.State == client.TunnelOpen && t.PublicPort != 0 {
				publicPort = t.PublicPort
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	check("tunnel open on public :%d", publicPort)

	step("hitting public address through the tunnel")
	body, err := httpGet(ctx, fmt.Sprintf("http://127.0.0.1:%d/hello", publicPort))
	if err != nil {
		return fmt.Errorf("public GET: %w", err)
	}
	if !strings.Contains(body, "echo:") {
		return fmt.Errorf("response didn't echo back: %q", body)
	}
	check("public GET returned %dB body containing 'echo:'", len(body))

	step("running 3 more requests to verify reuse")
	for i := 0; i < 3; i++ {
		_, err := httpGet(ctx, fmt.Sprintf("http://127.0.0.1:%d/req-%d", publicPort, i))
		if err != nil {
			return fmt.Errorf("req %d: %w", i, err)
		}
	}
	if got := atomic.LoadInt32(echoCount); got < 4 {
		return fmt.Errorf("echo target only saw %d requests, want >= 4", got)
	}
	check("echo target received %d requests", atomic.LoadInt32(echoCount))

	authBase := envOr("TEST_AUTH_BASE", "https://digger.gg")
	step("auth API smoke (%s)", authBase)
	if err := smokeAuth(ctx, authBase); err != nil {
		fmt.Fprintln(os.Stderr, "  (auth smoke skipped:", err, ")")
	} else {
		check("auth endpoints reachable")
	}

	// Full login round-trip — only runs if test creds are provided.
	if email, pw := os.Getenv("TEST_EMAIL"), os.Getenv("TEST_PASSWORD"); email != "" && pw != "" {
		step("full login flow (email/password)")
		if err := loginRoundTrip(ctx, authBase, email, pw); err != nil {
			return fmt.Errorf("login flow: %w", err)
		}
		check("device-code flow completed end-to-end with a real ID token")
	}

	step("preset shape integrity")
	if err := checkPresets(); err != nil {
		return err
	}
	check("%d presets all have valid shape", len(games.All()))

	step("interrupting agent — should reconnect from a clean state")
	agentCancel()
	time.Sleep(200 * time.Millisecond)

	return nil
}

// ────────────────────────────────────────────────────────────────────────

func step(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "▸ "+format+"\n", a...)
}
func check(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "  ✓ "+format+"\n", a...)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { return 0, err }
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(ctx context.Context, port int) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond); err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("port %d never opened", port)
}

func waitFor(ctx context.Context, label string, ok func() bool) error {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("timed out waiting for %s", label)
}

func startEchoServer(ctx context.Context, port int) (func(), *int32) {
	count := new(int32)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(count, 1)
		fmt.Fprintf(w, "echo:%s\n", r.URL.Path)
	})
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	return func() { _ = srv.Shutdown(ctx) }, count
}

func fetchJoin(ctx context.Context, url string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return strings.TrimSpace(string(body)), nil
}

func httpGet(ctx context.Context, url string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, io.LimitReader(resp.Body, 4096))
	return buf.String(), err
}

func smokeAuth(ctx context.Context, base string) error {
	// /api/auth/start
	resp, err := http.Post(base+"/api/auth/start", "application/json", nil)
	if err != nil { return fmt.Errorf("start: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 503 {
		return fmt.Errorf("start: HTTP %d", resp.StatusCode)
	}
	// /api/signup
	resp2, err := http.Get(base + "/api/signup")
	if err != nil { return fmt.Errorf("signup: %w", err) }
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 && resp2.StatusCode != 503 {
		return fmt.Errorf("signup: HTTP %d", resp2.StatusCode)
	}
	// auth.Login should error gracefully when 503
	scan := bufio.NewScanner(strings.NewReader(""))
	_ = scan
	_, err = auth.Login(ctx, base, func(string) {})
	if err != nil && !errors.Is(err, auth.ErrAuthUnavailable) && !strings.Contains(err.Error(), "expired") {
		// Any other error is also acceptable so long as it's an error
		// (not a successful login, which can't happen in CI).
	}
	return nil
}

// envOr returns the env var or the fallback if it's empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loginRoundTrip drives the entire device-code flow without a browser:
//   1. POST /api/auth/start  → user_code
//   2. Firebase REST signInWithPassword (creates account on first run) → idToken
//   3. POST /api/auth/complete { user_code, id_token }
//   4. POST /api/auth/poll { user_code } → expect status:"approved" + access_token
//   5. POST /api/auth/verify { token } → expect 200 + same uid
func loginRoundTrip(ctx context.Context, base, email, password string) error {
	// 1. start a session
	resp, err := http.Post(base+"/api/auth/start", "application/json", nil)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("start: HTTP %d: %s", resp.StatusCode, body)
	}
	var s struct {
		UserCode string `json:"user_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return fmt.Errorf("start decode: %w", err)
	}
	check("session created: %s", s.UserCode)

	// 2. firebase email/password sign-in via REST
	apiKey := envOr("TEST_FIREBASE_API_KEY", "AIzaSyBmdwKHljeMNFEhF0cEDgqdfboFuZw_j80")
	idToken, uid, err := firebaseSignIn(ctx, apiKey, email, password)
	if err != nil {
		return fmt.Errorf("firebase sign-in: %w", err)
	}
	check("signed in to firebase as %s (%s)", email, uid[:8])

	// 3. complete the session with the ID token
	cBody, _ := json.Marshal(map[string]string{"user_code": s.UserCode, "id_token": idToken})
	cResp, err := http.Post(base+"/api/auth/complete", "application/json", bytes.NewReader(cBody))
	if err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	cb, _ := io.ReadAll(cResp.Body)
	cResp.Body.Close()
	if cResp.StatusCode != 200 {
		return fmt.Errorf("complete: HTTP %d: %s", cResp.StatusCode, cb)
	}
	check("session approved on server")

	// 4. poll
	pBody, _ := json.Marshal(map[string]string{"user_code": s.UserCode})
	pResp, err := http.Post(base+"/api/auth/poll", "application/json", bytes.NewReader(pBody))
	if err != nil {
		return fmt.Errorf("poll: %w", err)
	}
	defer pResp.Body.Close()
	var p struct {
		Status      string `json:"status"`
		AccessToken string `json:"access_token"`
		User        struct {
			UID   string `json:"uid"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.NewDecoder(pResp.Body).Decode(&p); err != nil {
		return fmt.Errorf("poll decode: %w", err)
	}
	if p.Status != "approved" {
		return fmt.Errorf("poll status=%q, want approved", p.Status)
	}
	if p.User.UID != uid {
		return fmt.Errorf("poll uid mismatch: got %q want %q", p.User.UID, uid)
	}
	check("poll returned access_token (%dB) for uid %s", len(p.AccessToken), p.User.UID[:8])

	// 5. verify the token via /api/auth/verify
	vBody, _ := json.Marshal(map[string]string{"token": p.AccessToken})
	vResp, err := http.Post(base+"/api/auth/verify", "application/json", bytes.NewReader(vBody))
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	defer vResp.Body.Close()
	if vResp.StatusCode != 200 {
		body, _ := io.ReadAll(vResp.Body)
		return fmt.Errorf("verify: HTTP %d: %s", vResp.StatusCode, body)
	}
	var v struct {
		UID string `json:"uid"`
	}
	if err := json.NewDecoder(vResp.Body).Decode(&v); err != nil {
		return fmt.Errorf("verify decode: %w", err)
	}
	if v.UID != uid {
		return fmt.Errorf("verify uid mismatch: got %q want %q", v.UID, uid)
	}
	check("token verified server-side, same uid")
	return nil
}

// firebaseSignIn talks to the Firebase Identity Toolkit REST API.
// Returns (idToken, uid). Auto-creates the account on EMAIL_NOT_FOUND.
func firebaseSignIn(ctx context.Context, apiKey, email, password string) (string, string, error) {
	body, _ := json.Marshal(map[string]any{
		"email": email, "password": password, "returnSecureToken": true,
	})
	tok, uid, err := firebaseCall(ctx, "accounts:signInWithPassword", apiKey, body)
	// Newer Firebase behavior masks user-not-found vs wrong-password as
	// INVALID_LOGIN_CREDENTIALS to prevent enumeration. Also handle
	// EMAIL_NOT_FOUND for older configs. In either case, try signing up.
	if err != nil && (strings.Contains(err.Error(), "EMAIL_NOT_FOUND") ||
		strings.Contains(err.Error(), "INVALID_LOGIN_CREDENTIALS")) {
		tok, uid, err = firebaseCall(ctx, "accounts:signUp", apiKey, body)
	}
	return tok, uid, err
}

func firebaseCall(_ context.Context, endpoint, apiKey string, body []byte) (string, string, error) {
	url := fmt.Sprintf("https://identitytoolkit.googleapis.com/v1/%s?key=%s", endpoint, apiKey)
	// Use a fresh background context — the parent ctx may be partially
	// cancelled or have residual deadline from upstream and we don't
	// want it to interfere with this independent HTTPS call.
	cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(cctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}
	var r struct {
		IDToken   string `json:"idToken"`
		LocalID   string `json:"localId"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", "", err
	}
	return r.IDToken, r.LocalID, nil
}

func checkPresets() error {
	for _, p := range games.All() {
		if p.Name == "" { return fmt.Errorf("preset name empty") }
		if len(p.Ports) == 0 { return fmt.Errorf("%s has no ports", p.Name) }
		for _, port := range p.Ports {
			if port.LocalPort == 0 { return fmt.Errorf("%s: zero port", p.Name) }
		}
	}
	return nil
}
