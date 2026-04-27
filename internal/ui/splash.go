package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/digger-gg/digger-client/internal/theme"
)

// splashState — picks an initial theme and shows the brand.
type splashState struct {
	frame int // animation frame counter (cosmetic, unused for now)
}

func newSplash() splashState { return splashState{} }

const logoArt = `
 ██████╗ ██╗      █████╗ ██╗   ██╗██╗████████╗
 ██╔══██╗██║     ██╔══██╗╚██╗ ██╔╝██║╚══██╔══╝
 ██████╔╝██║     ███████║ ╚████╔╝ ██║   ██║
 ██╔═══╝ ██║     ██╔══██║  ╚██╔╝  ██║   ██║
 ██║     ███████╗██║  ██║   ██║   ██║   ██║
 ╚═╝     ╚══════╝╚═╝  ╚═╝   ╚═╝   ╚═╝   ╚═╝
            ─── lite ───
`

func (a App) updateSplash(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "left", "h":
		return a.cycleTheme(-1), nil
	case "right", "l", "tab":
		return a.cycleTheme(1), nil
	case "q", "esc":
		return a, tea.Quit
	case "enter", " ":
		a.screen = screenSetup
		return a, nil
	}
	return a, nil
}

func (a App) viewSplash() string {
	s := a.styles()
	logo := s.Accent.Render(logoArt)
	tagline := s.Accent2.Render("self-hosted tunnel — no website, no account")

	themePicker := a.renderThemePicker()
	help := s.Subtle.Render("← →  cycle theme       enter  begin       q  quit")

	stack := lipgloss.JoinVertical(lipgloss.Center,
		"",
		logo,
		"",
		tagline,
		"",
		"",
		themePicker,
		"",
		help,
	)
	return centerOnBlank(a.width, a.height, stack)
}

func (a App) renderThemePicker() string {
	s := a.styles()
	all := theme.All()

	var items []string
	for i, t := range all {
		swatch := renderSwatch(t)
		name := t.Name
		row := swatch + "  " + name
		if i == a.themeIdx {
			row = s.Selected.Render(" ▸ "+name+" ") + "  " + swatch
		} else {
			row = "   " + s.Subtle.Render(name) + "  " + swatch
		}
		items = append(items, row)
	}
	box := s.Box.
		BorderForeground(a.theme.Border).
		Padding(1, 3).
		Render(s.BoxTitle.Render("theme") + "\n\n" + strings.Join(items, "\n"))
	return box
}

func renderSwatch(t theme.Theme) string {
	mk := func(c lipgloss.Color) string {
		return lipgloss.NewStyle().Foreground(c).Render("●")
	}
	return mk(t.Accent) + " " + mk(t.Accent2) + " " + mk(t.Success) + " " + mk(t.Warning) + " " + mk(t.Error) + " " + mk(t.Info)
}
