package cfg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseJoin(t *testing.T) {
	cases := []struct {
		in        string
		wantRelay string
		wantSec   string
		wantErr   bool
	}{
		{"pl1://t123@1.2.3.4:7777", "1.2.3.4:7777", "t123", false},
		{"pl1://1.2.3.4:7777", "1.2.3.4:7777", "", false},
		{"pl1://abcd-1234@example.com:7777", "example.com:7777", "abcd-1234", false},
		{"http://wrong-scheme", "", "", true},
		{"pl1://", "", "", true},
		{"", "", "", true},
		{"not a url", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r, s, err := ParseJoin(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got relay=%q secret=%q", r, s)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r != c.wantRelay {
				t.Errorf("relay: got %q, want %q", r, c.wantRelay)
			}
			if s != c.wantSec {
				t.Errorf("secret: got %q, want %q", s, c.wantSec)
			}
		})
	}
}

func TestFormatJoin_RoundTrip(t *testing.T) {
	cases := []struct {
		relay  string
		secret string
	}{
		{"1.2.3.4:7777", "abc-123"},
		{"example.com:9999", ""},
		{"[::1]:7777", "secret"},
	}
	for _, c := range cases {
		t.Run(c.relay, func(t *testing.T) {
			j := FormatJoin(c.relay, c.secret)
			r, s, err := ParseJoin(j)
			if err != nil {
				t.Fatalf("round-trip parse: %v (formatted as %q)", err, j)
			}
			if r != c.relay || s != c.secret {
				t.Errorf("round-trip: in (%q,%q) → %q → out (%q,%q)",
					c.relay, c.secret, j, r, s)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	in := Config{
		Relay:     "1.2.3.4:7777",
		Secret:    "shh",
		Theme:     "tokyo night",
		Token:     "tok",
		UserUID:   "u1",
		UserEmail: "u@e.com",
		UserName:  "U",
	}
	if err := Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch:\n  in:  %+v\n  out: %+v", in, out)
	}
	// File must be 0600 — it carries a secret + auth token.
	p, _ := Path()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Errorf("config perms: got %o, want 0600", mode)
	}
}

func TestPath_UsesXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	p, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join("/tmp/xdg-test", "digger", "config.toml")
	if p != want {
		t.Errorf("Path: got %q, want %q", p, want)
	}
}

func TestFetchSignup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte("pl1://t@1.2.3.4:7777\n"))
		case "/empty":
			w.Write([]byte(""))
		case "/notfound":
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	if got, err := FetchSignup(srv.URL + "/ok"); err != nil || got != "pl1://t@1.2.3.4:7777" {
		t.Errorf("ok: got %q err=%v", got, err)
	}
	if got, err := FetchSignup(srv.URL + "/empty"); err != nil || got != "" {
		t.Errorf("empty: got %q err=%v", got, err)
	}
	if _, err := FetchSignup(srv.URL + "/notfound"); err == nil {
		t.Error("notfound: expected error")
	}
	if _, err := FetchSignup(""); err == nil {
		t.Error("empty url: expected error")
	}
}
