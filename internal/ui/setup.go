package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/digger-gg/digger-client/internal/cfg"
	"github.com/digger-gg/digger-client/internal/client"
)

// setupState — single field: paste a join string from the relay.
type setupState struct {
	join string
	err  string
}

func newSetup() setupState { return setupState{} }

func (a App) updateSetup(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "esc":
		a.screen = screenSplash
		return a, nil
	case "enter":
		return a.startConnectFromJoin()
	case "backspace":
		if len(a.setup.join) > 0 {
			a.setup.join = a.setup.join[:len(a.setup.join)-1]
		}
		return a, nil
	case "ctrl+u":
		a.setup.join = ""
		return a, nil
	}
	if len(m.Runes) > 0 {
		a.setup.join += string(m.Runes)
	}
	return a, nil
}

func (a App) startConnectFromJoin() (tea.Model, tea.Cmd) {
	join := strings.TrimSpace(a.setup.join)
	if join == "" {
		a.setup.err = "paste the join string from the relay"
		return a, nil
	}
	relay, secret, err := cfg.ParseJoin(join)
	if err != nil {
		a.setup.err = err.Error()
		return a, nil
	}
	// persist for next launch
	_ = cfg.Save(cfg.Config{Relay: relay, Secret: secret, Theme: a.theme.Name})
	return a.startWith(relay, secret)
}

func (a App) startWith(relay, secret string) (tea.Model, tea.Cmd) {
	c := client.New(client.Config{Relay: relay, Secret: secret})
	ctx, cancel := context.WithCancel(context.Background())
	a.client = c
	a.clientCtx = ctx
	a.cancel = cancel
	go func() { _ = c.Run(ctx) }()
	a.snapshot = c.Snapshot()
	a.screen = screenMain
	// Land directly on game-preset selection — that's the most common
	// next action. User can press esc to dismiss and add custom tunnels.
	a.modal = modalPresets
	a.add.preset = 0
	return a, nil
}

func (a App) viewSetup() string {
	s := a.styles()
	caret := s.Accent.Render("▍")
	inputBox := s.InputFocus.Width(64).Render(a.setup.join + caret)

	help := s.Subtle.Render("paste the join string the relay printed at startup")
	example := s.Subtle.Render("e.g.  ") + s.Accent2.Render("pl1://abcd-1234@1.2.3.4:7777")
	keys := s.Subtle.Render("enter  connect    ctrl+u  clear    esc  back")

	var errLine string
	if a.setup.err != "" {
		errLine = s.Error.Render(a.setup.err)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		s.Title.Render("paste join string"),
		help,
		"",
		inputBox,
		"",
		example,
		"",
		errLine,
	)
	box := s.Box.
		BorderForeground(a.theme.Accent).
		Padding(1, 3).
		Render(body)
	return centerOnBlank(a.width, a.height,
		lipgloss.JoinVertical(lipgloss.Center, box, "", keys))
}
