// Package ui hosts the bubbletea application: splash → setup → main with
// modals for game-preset picking, custom-tunnel entry, and confirm.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/digger-gg/digger-client/internal/client"
	"github.com/digger-gg/digger-client/internal/games"
	"github.com/digger-gg/digger-client/internal/theme"
	"github.com/digger-gg/digger-client/proto"
)

type screen int

const (
	screenSplash screen = iota
	screenSetup
	screenMain
)

type modal int

const (
	modalNone modal = iota
	modalPresets
	modalCustom
	modalConfirm
)

// App is the root TUI model.
type App struct {
	width, height int
	theme         theme.Theme
	themeIdx      int

	screen screen
	modal  modal

	splash splashState
	setup  setupState
	main   mainState
	add    addState
	confirm confirmState

	client    *client.Client
	clientCtx context.Context
	cancel    context.CancelFunc
	snapshot  client.Snapshot
}

func New() App {
	themes := theme.All()
	t := themes[0]
	app := App{
		width:    100,
		height:   30,
		theme:    t,
		themeIdx: 0,
		screen:   screenSplash,
		splash:   newSplash(),
		setup:    newSetup(),
		main:     newMain(),
		add:      newAdd(),
	}
	return app
}

func (a App) Init() tea.Cmd { return tea.Batch(tea.EnterAltScreen, tickCmd()) }

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil
	case tickMsg:
		if a.client != nil {
			a.snapshot = a.client.Snapshot()
		}
		return a, tickCmd()
	case tea.KeyMsg:
		// global: ctrl+c always quits
		if m.Type == tea.KeyCtrlC {
			if a.cancel != nil {
				a.cancel()
			}
			return a, tea.Quit
		}
		// modals capture input first
		if a.modal != modalNone {
			return a.updateModal(m)
		}
		switch a.screen {
		case screenSplash:
			return a.updateSplash(m)
		case screenSetup:
			return a.updateSetup(m)
		case screenMain:
			return a.updateMain(m)
		}
	}
	return a, nil
}

func (a App) View() string {
	if a.width == 0 || a.height == 0 {
		return ""
	}
	var body string
	switch a.screen {
	case screenSplash:
		body = a.viewSplash()
	case screenSetup:
		body = a.viewSetup()
	case screenMain:
		body = a.viewMain()
	}
	if a.modal != modalNone {
		body = a.overlayModal(body)
	}
	// Don't repaint the whole terminal with a theme background — the alt
	// screen handles that, and our own paint produces stripes wherever
	// inner widgets don't fully cover their slot. Just set foreground.
	return lipgloss.NewStyle().Foreground(a.theme.Foreground).Render(body)
}

// ---- helpers shared across screens --------------------------------------

func (a App) styles() theme.Styles { return a.theme.Styles() }

func (a App) cycleTheme(delta int) App {
	all := theme.All()
	a.themeIdx = (a.themeIdx + delta + len(all)) % len(all)
	a.theme = all[a.themeIdx]
	return a
}

func (a App) overlayModal(base string) string {
	var modal string
	switch a.modal {
	case modalPresets:
		modal = a.viewPresets()
	case modalCustom:
		modal = a.viewAdd()
	case modalConfirm:
		modal = a.viewConfirm()
	}
	return placeOverlay(a.width, a.height, base, modal)
}

// placeOverlay centers `top` on a blank canvas of size w×h.
//
// We avoid lipgloss.Place — it fills the surrounding cells with a default
// style which renders as a gray block on most terminals when the alt
// screen has its own background. Instead we manually pad with newlines
// (top) and spaces (left), so the surrounding area shows through as the
// terminal's actual background.
func placeOverlay(w, h int, _, top string) string {
	tw, th := lipgloss.Size(top)
	if tw >= w || th >= h {
		return top
	}
	padTop := (h - th) / 2
	padLeft := (w - tw) / 2
	var b strings.Builder
	for i := 0; i < padTop; i++ {
		b.WriteByte('\n')
	}
	leftPad := strings.Repeat(" ", padLeft)
	for _, line := range strings.Split(top, "\n") {
		b.WriteString(leftPad)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func humanBytes(n uint64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"K", "M", "G", "T"}
	v := float64(n) / k
	idx := 0
	for v >= k && idx < len(units)-1 {
		v /= k
		idx++
	}
	return fmt.Sprintf("%.1f%sB", v, units[idx])
}

// centerOnBlank places content roughly centered with newline+space padding,
// avoiding lipgloss.Place's gray fill cells.
func centerOnBlank(w, h int, content string) string {
	tw, th := lipgloss.Size(content)
	padTop := (h - th) / 2
	if padTop < 0 {
		padTop = 0
	}
	padLeft := (w - tw) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	var b strings.Builder
	for i := 0; i < padTop; i++ {
		b.WriteByte('\n')
	}
	leftPad := strings.Repeat(" ", padLeft)
	for _, line := range strings.Split(content, "\n") {
		b.WriteString(leftPad)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func dim(s theme.Styles, lines ...string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = s.Subtle.Render(l)
	}
	return strings.Join(out, "\n")
}

// ---- entry point --------------------------------------------------------

// RunOptions configure the TUI launch.
type RunOptions struct {
	// If both non-empty, skip splash & setup, start connected.
	InitialRelay  string
	InitialSecret string
	// If non-empty, select this theme by name.
	StartingThemeName string
}

func Run(opts RunOptions, _ []games.Preset) error {
	a := New()
	if opts.StartingThemeName != "" {
		all := theme.All()
		for i, t := range all {
			if t.Name == opts.StartingThemeName {
				a.themeIdx = i
				a.theme = t
				break
			}
		}
	}
	if opts.InitialRelay != "" {
		// jump straight into the main screen with an active client.
		mdl, _ := a.startWith(opts.InitialRelay, opts.InitialSecret)
		a = mdl.(App)
	}
	prog := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := prog.Run()
	return err
}

var _ = proto.Tcp
var _ = client.New
