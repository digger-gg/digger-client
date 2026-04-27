// digger — TUI client for digger.gg.
//
// Subcommands:
//
//	digger              launch the TUI (logging in if needed)
//	digger login        device-code flow; saves token + identity
//	digger logout       wipe saved config
//	digger version      print version
//
// Behind the scenes the TUI bootstraps a tunnel as before:
//   1. positional join string  (digger pl1://...)
//   2. --signup URL or PLAYIT_SIGNUP env
//   3. saved config (~/.config/digger/config.toml)
//   4. build-time defaults (-X main.defaultJoin / main.defaultSignup)
//   5. interactive paste screen
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/digger-gg/digger-client/internal/auth"
	"github.com/digger-gg/digger-client/internal/cfg"
	"github.com/digger-gg/digger-client/internal/ui"
)

// build-time defaults (override with `go build -ldflags`)
var (
	defaultJoin   = ""
	defaultSignup = "http://digger.gg:7778/"
	defaultAuth   = "https://digger.gg"
	version       = "v0.4.0"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "login":
			os.Exit(cmdLogin(os.Args[2:]))
		case "logout":
			os.Exit(cmdLogout())
		case "version", "--version", "-v":
			fmt.Println(version)
			return
		case "help", "--help", "-h":
			fmt.Print(usage)
			return
		}
	}
	cmdRun(os.Args[1:])
}

const usage = `digger -- host a game server, anywhere

usage:
  digger                            launch the TUI
  digger login                      sign in via browser, save the token
  digger logout                     wipe saved credentials
  digger version

  digger pl1://secret@host:port     start with a specific relay
  digger --signup URL               fetch join string from a relay
`

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	signupFlag := fs.String("signup", "", "signup URL — fetch a join string then save")
	skipAuth := fs.Bool("no-auth", false, "skip the sign-in step (for development)")
	fs.Parse(args)

	opts := ui.RunOptions{}

	join := ""
	if len(fs.Args()) > 0 {
		join = fs.Arg(0)
	} else if v := os.Getenv("PLAYIT_JOIN"); v != "" {
		join = v
	}
	signupURL := *signupFlag
	if signupURL == "" {
		signupURL = os.Getenv("PLAYIT_SIGNUP")
	}

	loaded, _ := cfg.Load()

	// Auto-login if we don't have an identity yet. The user can opt out
	// with --no-auth or by setting an explicit join string on the cmdline.
	if !*skipAuth && loaded.Token == "" && join == "" {
		if err := runLogin(&loaded); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	switch {
	case join != "":
		applyJoin(join, &opts, &loaded, true)
	case signupURL != "":
		if err := bootstrapFromSignup(signupURL, &opts, &loaded); err != nil {
			fmt.Fprintf(os.Stderr, "signup failed: %v\n", err)
			os.Exit(2)
		}
	default:
		if loaded.Relay != "" {
			opts.InitialRelay = loaded.Relay
			opts.InitialSecret = loaded.Secret
			opts.StartingThemeName = loaded.Theme
		} else if defaultJoin != "" {
			applyJoin(defaultJoin, &opts, &loaded, true)
		} else if defaultSignup != "" {
			_ = bootstrapFromSignup(defaultSignup, &opts, &loaded)
		}
	}

	opts.UserName = loaded.UserName
	opts.UserEmail = loaded.UserEmail
	opts.UserUID = loaded.UserUID

	if err := ui.Run(opts, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runLogin performs the device-code flow and updates `c` in place. Used
// both by the standalone `digger login` subcommand and by the auto-login
// step at the start of `digger`.
func runLogin(c *cfg.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	fmt.Println("digger — sign in to continue")
	res, err := auth.Login(ctx, defaultAuth, func(line string) {
		fmt.Println(line)
	})
	if err != nil {
		return fmt.Errorf("sign-in failed: %w", err)
	}
	c.Token = res.Token
	c.UserUID = res.User.UID
	c.UserEmail = res.User.Email
	c.UserName = res.User.DisplayName
	c.UserPicture = res.User.PhotoURL
	return cfg.Save(*c)
}

func cmdLogin(args []string) int {
	c, _ := cfg.Load()
	if err := runLogin(&c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("  saved to ~/.config/digger/config.toml")
	return 0
}

func cmdLogout() int {
	c, _ := cfg.Load()
	c.Token = ""
	c.UserUID = ""
	c.UserEmail = ""
	c.UserName = ""
	c.UserPicture = ""
	if err := cfg.Save(c); err != nil {
		// even if save fails, fall back to deleting the whole file
		if p, e := cfg.Path(); e == nil {
			_ = os.Remove(p)
		}
	}
	fmt.Println("logged out")
	return 0
}

func applyJoin(join string, opts *ui.RunOptions, c *cfg.Config, save bool) {
	relay, secret, err := cfg.ParseJoin(join)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad join string: %v\n", err)
		os.Exit(2)
	}
	opts.InitialRelay = relay
	opts.InitialSecret = secret
	if save {
		c.Relay = relay
		c.Secret = secret
		_ = cfg.Save(*c)
	}
}

func bootstrapFromSignup(url string, opts *ui.RunOptions, c *cfg.Config) error {
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
	c.Relay = relay
	c.Secret = secret
	_ = cfg.Save(*c)
	return nil
}
