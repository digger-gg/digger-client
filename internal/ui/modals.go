package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/digger-gg/digger-client/internal/client"
	"github.com/digger-gg/digger-client/internal/games"
	"github.com/digger-gg/digger-client/proto"
)

// ──────────────────────────────────────────────────────────────────────
// game presets modal

func (a App) viewPresets() string {
	s := a.styles()
	all := games.All()
	rows := []string{s.BoxTitle.Render("pick a game"), ""}
	for i, p := range all {
		marker := "   "
		nameStyle := lipgloss.NewStyle().Foreground(a.theme.Foreground)
		if i == a.add.preset {
			marker = s.Accent.Render(" ▸ ")
			nameStyle = s.Selected
		}
		line := marker + nameStyle.Render(padRight(p.Name, 22)) + "  " + s.Subtle.Render(p.Note)
		rows = append(rows, line)
	}
	rows = append(rows, "", s.Subtle.Render("↑/↓  pick    enter  add tunnels    esc  cancel"))
	body := strings.Join(rows, "\n")
	return s.Box.BorderForeground(a.theme.Accent).Padding(1, 3).Render(body)
}

// ──────────────────────────────────────────────────────────────────────
// add custom tunnel modal

type addField int

const (
	addProto addField = iota
	addPublicPort
	addLocalHost
	addLocalPort
	addSubmit
)

type addState struct {
	preset     int
	field      addField
	protoTcp   bool
	publicPort string
	localHost  string
	localPort  string
	err        string
}

func newAdd() addState {
	return addState{
		protoTcp:  true,
		localHost: "127.0.0.1",
	}
}

func (a App) viewAdd() string {
	s := a.styles()

	protoLabel := "tcp"
	if !a.add.protoTcp {
		protoLabel = "udp"
	}
	field := func(label, value string, focused bool) string {
		boxStyle := s.InputField
		labelStyle := s.Subtle
		if focused {
			boxStyle = s.InputFocus
			labelStyle = s.Accent
		}
		caret := ""
		if focused {
			caret = s.Accent.Render("▍")
		}
		return labelStyle.Render(label) + "\n" +
			boxStyle.Width(36).Render(value+caret)
	}
	protoBox := func() string {
		labelStyle := s.Subtle
		if a.add.field == addProto {
			labelStyle = s.Accent
		}
		hint := s.Subtle.Render(" (←/→ or space)")
		return labelStyle.Render("protocol") + hint + "\n" +
			s.InputFocus.Width(36).Render(protoLabel)
	}()
	pubBox := field("public port  (leave blank for auto)", a.add.publicPort, a.add.field == addPublicPort)
	hostBox := field("local host", a.add.localHost, a.add.field == addLocalHost)
	portBox := field("local port", a.add.localPort, a.add.field == addLocalPort)

	startStyle := s.InputField.Padding(0, 2)
	if a.add.field == addSubmit {
		startStyle = s.Selected.Padding(0, 2)
	}
	addBtn := startStyle.Render("  add  ")

	var errLine string
	if a.add.err != "" {
		errLine = s.Error.Render(a.add.err)
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		s.BoxTitle.Render("custom tunnel"),
		"",
		protoBox,
		"",
		pubBox,
		"",
		hostBox,
		"",
		portBox,
		"",
		addBtn,
		"",
		errLine,
		"",
		s.Subtle.Render("tab/shift-tab  field   enter  add   esc  cancel"),
	)
	return s.Box.BorderForeground(a.theme.Accent).Padding(1, 3).Render(body)
}

// ──────────────────────────────────────────────────────────────────────
// confirm-delete modal

type confirmState struct {
	tid uint32
}

func (a App) viewConfirm() string {
	s := a.styles()
	body := lipgloss.JoinVertical(lipgloss.Center,
		s.BoxTitle.Render("delete tunnel?"),
		"",
		s.Subtle.Render(fmt.Sprintf("tunnel id %d will be closed", a.confirm.tid)),
		"",
		s.Subtle.Render("y  yes      n / esc  cancel"),
	)
	return s.Box.BorderForeground(a.theme.Error).Padding(1, 3).Render(body)
}

// ──────────────────────────────────────────────────────────────────────
// modal input dispatch

func (a App) updateModal(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.modal {
	case modalPresets:
		return a.updatePresets(m)
	case modalCustom:
		return a.updateAdd(m)
	case modalConfirm:
		return a.updateConfirm(m)
	}
	return a, nil
}

func (a App) updatePresets(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	all := games.All()
	switch m.String() {
	case "esc":
		a.modal = modalNone
		return a, nil
	case "down", "j":
		if a.add.preset+1 < len(all) {
			a.add.preset++
		}
		return a, nil
	case "up", "k":
		if a.add.preset > 0 {
			a.add.preset--
		}
		return a, nil
	case "enter":
		preset := all[a.add.preset]
		if a.client != nil {
			for _, ps := range preset.Ports {
				a.client.Send(client.CmdAddTunnel{
					Proto:      ps.Proto,
					PublicPort: ps.PublicPort,
					LocalAddr:  fmt.Sprintf("127.0.0.1:%d", ps.LocalPort),
				})
			}
		}
		a.modal = modalNone
		return a, nil
	}
	return a, nil
}

func (a App) updateAdd(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "esc":
		a.modal = modalNone
		return a, nil
	case "tab", "down":
		a.add.field = (a.add.field + 1) % 5
		return a, nil
	case "shift+tab", "up":
		a.add.field = (a.add.field + 4) % 5
		return a, nil
	case "left", "right", " ":
		if a.add.field == addProto {
			a.add.protoTcp = !a.add.protoTcp
		}
		return a, nil
	case "enter":
		if a.add.field == addSubmit || a.add.field == addLocalPort {
			return a.submitAdd()
		}
		a.add.field++
		return a, nil
	case "backspace":
		switch a.add.field {
		case addPublicPort:
			a.add.publicPort = trimLast(a.add.publicPort)
		case addLocalHost:
			a.add.localHost = trimLast(a.add.localHost)
		case addLocalPort:
			a.add.localPort = trimLast(a.add.localPort)
		}
		return a, nil
	}
	if len(m.Runes) > 0 {
		switch a.add.field {
		case addPublicPort:
			a.add.publicPort += string(m.Runes)
		case addLocalHost:
			a.add.localHost += string(m.Runes)
		case addLocalPort:
			a.add.localPort += string(m.Runes)
		}
	}
	return a, nil
}

func (a App) submitAdd() (tea.Model, tea.Cmd) {
	var pub uint16 = 0
	if s := strings.TrimSpace(a.add.publicPort); s != "" {
		n, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			a.add.err = "public port must be 0-65535 (or blank for auto)"
			return a, nil
		}
		pub = uint16(n)
	}
	host := strings.TrimSpace(a.add.localHost)
	if host == "" {
		host = "127.0.0.1"
	}
	lp, err := strconv.ParseUint(strings.TrimSpace(a.add.localPort), 10, 16)
	if err != nil || lp == 0 {
		a.add.err = "local port required (1-65535)"
		return a, nil
	}
	pr := proto.Tcp
	if !a.add.protoTcp {
		pr = proto.Udp
	}
	if a.client != nil {
		a.client.Send(client.CmdAddTunnel{
			Proto:      pr,
			PublicPort: pub,
			LocalAddr:  fmt.Sprintf("%s:%d", host, lp),
		})
	}
	a.modal = modalNone
	return a, nil
}

func (a App) updateConfirm(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "y", "enter":
		if a.client != nil {
			a.client.Send(client.CmdRemoveTunnel{Tid: a.confirm.tid})
		}
		a.modal = modalNone
		return a, nil
	case "n", "esc":
		a.modal = modalNone
		return a, nil
	}
	return a, nil
}

// ──────────────────────────────────────────────────────────────────────
// helpers

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func trimLast(s string) string {
	if s == "" {
		return s
	}
	return s[:len(s)-1]
}
