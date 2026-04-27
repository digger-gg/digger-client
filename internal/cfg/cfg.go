// Package cfg loads and saves persistent client config.
//
// Config lives at $XDG_CONFIG_HOME/digger/config.toml (or
// ~/.config/digger/config.toml). It stores the relay address,
// secret, and chosen theme. Once written, the user never sees the
// setup screen again.
package cfg

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Relay  string `toml:"relay"`
	Secret string `toml:"secret"`
	Theme  string `toml:"theme"`
}

func dir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "digger"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "digger"), nil
}

func Path() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.toml"), nil
}

func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	var c Config
	if _, err := toml.DecodeFile(p, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func Save(c Config) error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	p := filepath.Join(d, "config.toml")
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

// ParseJoin parses a join string of the form
//
//	pl1://secret@host:port
//	pl1://host:port            (no secret)
//
// and returns (relay, secret).
func ParseJoin(s string) (relay string, secret string, err error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("not a valid join string: %w", err)
	}
	if u.Scheme != "pl1" {
		return "", "", errors.New("expected pl1:// scheme")
	}
	host := u.Host
	if host == "" {
		// pl1://host:port  is parsed with host in u.Opaque on some Go versions
		host = u.Opaque
	}
	if host == "" {
		return "", "", errors.New("missing host:port")
	}
	if u.User != nil {
		secret = u.User.Username()
	}
	return host, secret, nil
}

// FormatJoin produces a join string from a relay address + optional secret.
func FormatJoin(relay, secret string) string {
	if secret == "" {
		return "pl1://" + relay
	}
	return fmt.Sprintf("pl1://%s@%s", url.QueryEscape(secret), relay)
}

// FetchSignup performs a short GET on signupURL and returns the body
// (expected to be a join string).
func FetchSignup(signupURL string) (string, error) {
	if signupURL == "" {
		return "", errors.New("empty signup URL")
	}
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(signupURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("signup returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}
