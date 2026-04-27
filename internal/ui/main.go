package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/digger-gg/digger-client/internal/client"
)

type mainState struct {
	cursor int // selected tunnel index
}

func newMain() mainState { return mainState{} }

func (a App) updateMain(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "q":
		if a.cancel != nil {
			a.cancel()
		}
		return a, tea.Quit
	case "g":
		a.modal = modalPresets
		a.add.preset = 0
		return a, nil
	case "a":
		a.modal = modalCustom
		a.add.field = addProto
		a.add.protoTcp = true
		a.add.publicPort = "" // 0 by default
		a.add.localHost = "127.0.0.1"
		a.add.localPort = ""
		a.add.err = ""
		return a, nil
	case "d", "delete":
		if len(a.snapshot.Tunnels) > 0 && a.main.cursor < len(a.snapshot.Tunnels) {
			a.confirm.tid = a.snapshot.Tunnels[a.main.cursor].Tid
			a.modal = modalConfirm
		}
		return a, nil
	case "down", "j":
		if a.main.cursor+1 < len(a.snapshot.Tunnels) {
			a.main.cursor++
		}
		return a, nil
	case "up", "k":
		if a.main.cursor > 0 {
			a.main.cursor--
		}
		return a, nil
	case "t":
		return a.cycleTheme(1), nil
	}
	return a, nil
}

func (a App) viewMain() string {
	s := a.styles()
	header := a.renderHeader()
	tunnels := a.renderTunnels()
	logs := a.renderLogs()
	footer := s.Footer.Render("a  add custom    g  game preset    d  delete    t  theme    q  quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, "", tunnels, "", logs, "", footer)
}

func (a App) renderHeader() string {
	s := a.styles()
	snap := a.snapshot

	var statusStyle lipgloss.Style
	switch snap.Status {
	case client.StatusConnected:
		statusStyle = s.Success
	case client.StatusConnecting:
		statusStyle = s.Warning
	case client.StatusDenied:
		statusStyle = s.Error
	default:
		statusStyle = s.Error
	}
	statusText := snap.Status.String()
	if snap.StatusMsg != "" && snap.Status != client.StatusConnected {
		statusText += ": " + snap.StatusMsg
	}

	traffic := s.Subtle.Render(fmt.Sprintf("↑ %s   ↓ %s", humanBytes(snap.BytesUp), humanBytes(snap.BytesDown)))
	identity := s.Subtle.Render("not signed in  ·  digger login")
	if a.userEmail != "" {
		who := a.userName
		if who == "" {
			who = a.userEmail
		}
		identity = s.Accent2.Render("@ "+who) + s.Subtle.Render("  "+a.userEmail)
	}
	left := s.Title.Render("digger") + "  " + s.Subtle.Render("→ "+snap.RelayAddr) + "  " + statusStyle.Render("● "+statusText)
	right := identity + "    " + traffic
	headerLine := lipgloss.JoinHorizontal(lipgloss.Left, left, "    ", right)
	return s.Box.
		BorderForeground(a.theme.Accent).
		Width(a.contentWidth()).
		Render(headerLine)
}

func (a App) renderTunnels() string {
	s := a.styles()
	snap := a.snapshot
	if len(snap.Tunnels) == 0 {
		empty := s.Subtle.Render(
			"no tunnels yet.\n\npress  " +
				s.Accent.Render("g") + s.Subtle.Render("  to pick a game preset, or  ") +
				s.Accent.Render("a") + s.Subtle.Render("  for a custom tunnel."))
		return s.Box.Padding(2, 3).Render(s.BoxTitle.Render("tunnels") + "\n\n" + empty)
	}
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		colWidth(s.Subtle.Render("  proto"), 8),
		colWidth(s.Subtle.Render("public address"), 38),
		colWidth(s.Subtle.Render("→ local"), 24),
		colWidth(s.Subtle.Render("status"), 18),
		s.Subtle.Render("conns"),
	)
	rows := []string{header}
	for i, t := range snap.Tunnels {
		marker := "  "
		if i == a.main.cursor {
			marker = s.Accent.Render("▸ ")
		}
		var pubAddr string
		if t.State == client.TunnelOpen && t.Bound != "" {
			pubAddr = a.formatPublic(t)
		} else {
			pubAddr = s.Subtle.Render("—")
		}
		var status string
		switch t.State {
		case client.TunnelOpen:
			status = s.Success.Render("● open")
		case client.TunnelPending:
			status = s.Warning.Render("○ pending")
		case client.TunnelFailed:
			status = s.Error.Render("✕ " + truncate(t.Error, 20))
		}
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			marker,
			colWidth(string(t.Proto), 6),
			colWidth(pubAddr, 38),
			colWidth(t.LocalAddr, 24),
			colWidth(status, 18),
			fmt.Sprintf("%d", t.Conns),
		)
		rows = append(rows, row)
	}
	return s.Box.BorderForeground(a.theme.Border).
		Padding(0, 1).
		Width(a.contentWidth()).
		Render(s.BoxTitle.Render("tunnels") + "\n\n" + strings.Join(rows, "\n"))
}

func (a App) formatPublic(t client.Tunnel) string {
	s := a.styles()
	host := a.snapshot.RelayHost
	if host == "" {
		host = "<relay>"
	}
	addr := fmt.Sprintf("%s:%d", host, t.PublicPort)
	tag := s.Subtle.Render("(" + t.Slug + ")")
	return s.Accent.Render(addr) + " " + tag
}

func (a App) renderLogs() string {
	s := a.styles()
	logs := a.snapshot.Logs
	maxLines := a.height - 12
	if maxLines < 5 {
		maxLines = 5
	}
	if len(logs) > maxLines {
		logs = logs[len(logs)-maxLines:]
	}
	body := strings.Join(logs, "\n")
	if body == "" {
		body = s.Subtle.Render("(no log entries yet)")
	}
	return s.Box.BorderForeground(a.theme.Border).
		Padding(0, 1).
		Width(a.contentWidth()).
		Height(maxLines + 2).
		Render(s.BoxTitle.Render("activity") + "\n\n" + body)
}

// contentWidth returns the inner width all top-level boxes should share,
// capped so layouts stay legible on ultra-wide terminals.
func (a App) contentWidth() int {
	w := a.width - 2
	if w > 120 {
		w = 120
	}
	if w < 40 {
		w = 40
	}
	return w
}

func colWidth(s string, w int) string {
	return lipgloss.NewStyle().Width(w).Render(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}
