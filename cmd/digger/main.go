// digger — bubble-tea TUI client for digger.
//
// Bootstrapping order (first match wins):
//
//  1. positional arg or --signup flag:
//       digger pl1://secret@host:port            (join string)
//       digger --signup http://host:7778/        (signup URL, fetched once)
//
//  2. env vars  PLAYIT_JOIN, PLAYIT_SIGNUP
//
//  3. saved config at ~/.config/digger/config.toml
//
//  4. build-time embedded defaults (set with `go build -ldflags`)
//       -X 'main.defaultJoin=pl1://...'
//       -X 'main.defaultSignup=http://relay-host:7778/'
//
//  5. interactive paste screen.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/digger-gg/digger-client/internal/cfg"
	"github.com/digger-gg/digger-client/internal/ui"
)

// build-time defaults (override with `go build -ldflags`)
var (
	defaultJoin   = ""
	defaultSignup = "http://digger.gg:7778/"
)

func main() {
	signupFlag := flag.String("signup", "", "signup URL — fetch a join string then save")
	logout := flag.Bool("logout", false, "delete saved config and exit")
	flag.Parse()

	if *logout {
		p, err := cfg.Path()
		if err == nil {
			os.Remove(p)
		}
		fmt.Println("forgot saved relay")
		return
	}

	opts := ui.RunOptions{}

	join := ""
	if len(flag.Args()) > 0 {
		join = flag.Arg(0)
	} else if v := os.Getenv("PLAYIT_JOIN"); v != "" {
		join = v
	}

	signupURL := *signupFlag
	if signupURL == "" {
		signupURL = os.Getenv("PLAYIT_SIGNUP")
	}

	switch {
	case join != "":
		// 1a. explicit join string
		applyJoin(join, &opts, /*save*/ true)
	case signupURL != "":
		// 1b. explicit signup URL — fetch and import
		if err := bootstrapFromSignup(signupURL, &opts); err != nil {
			fmt.Fprintf(os.Stderr, "signup failed: %v\n", err)
			os.Exit(2)
		}
	default:
		if c, err := cfg.Load(); err == nil && c.Relay != "" {
			// 2. saved config
			opts.InitialRelay = c.Relay
			opts.InitialSecret = c.Secret
			opts.StartingThemeName = c.Theme
		} else if defaultJoin != "" {
			// 3a. build-time default join
			applyJoin(defaultJoin, &opts, true)
		} else if defaultSignup != "" {
			// 3b. build-time default signup — silent fallback if relay
			// isn't running locally; the TUI shows the paste screen.
			_ = bootstrapFromSignup(defaultSignup, &opts)
		}
		// 4. paste screen (handled by TUI when InitialRelay is empty)
	}

	if err := ui.Run(opts, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func applyJoin(join string, opts *ui.RunOptions, save bool) {
	relay, secret, err := cfg.ParseJoin(join)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad join string: %v\n", err)
		os.Exit(2)
	}
	opts.InitialRelay = relay
	opts.InitialSecret = secret
	if save {
		_ = cfg.Save(cfg.Config{Relay: relay, Secret: secret})
	}
}

func bootstrapFromSignup(url string, opts *ui.RunOptions) error {
	join, err := cfg.FetchSignup(url)
	if err != nil {
		return err
	}
	relay, secret, err := cfg.ParseJoin(join)
	if err != nil {
		return fmt.Errorf("signup body wasn't a join string: %w", err)
	}
	opts.InitialRelay = relay
	opts.InitialSecret = secret
	_ = cfg.Save(cfg.Config{Relay: relay, Secret: secret})
	return nil
}
