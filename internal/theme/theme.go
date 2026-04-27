// Package theme defines color palettes for the TUI.
package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name       string
	Background lipgloss.Color
	Foreground lipgloss.Color
	Subtle     lipgloss.Color
	Border     lipgloss.Color
	Accent     lipgloss.Color
	Accent2    lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Info       lipgloss.Color
}

var Catppuccin = Theme{
	Name:       "catppuccin mocha",
	Background: "#1e1e2e",
	Foreground: "#cdd6f4",
	Subtle:     "#7f849c",
	Border:     "#45475a",
	Accent:     "#cba6f7", // mauve
	Accent2:    "#fab387", // peach
	Success:    "#a6e3a1", // green
	Warning:    "#f9e2af", // yellow
	Error:      "#f38ba8", // red
	Info:       "#74c7ec", // sapphire
}

var TokyoNight = Theme{
	Name:       "tokyo night",
	Background: "#1a1b26",
	Foreground: "#c0caf5",
	Subtle:     "#565f89",
	Border:     "#414868",
	Accent:     "#7aa2f7", // blue
	Accent2:    "#bb9af7", // purple
	Success:    "#9ece6a",
	Warning:    "#e0af68",
	Error:      "#f7768e",
	Info:       "#7dcfff",
}

var Gruvbox = Theme{
	Name:       "gruvbox",
	Background: "#282828",
	Foreground: "#ebdbb2",
	Subtle:     "#928374",
	Border:     "#504945",
	Accent:     "#fe8019", // orange
	Accent2:    "#d79921", // yellow
	Success:    "#b8bb26",
	Warning:    "#fabd2f",
	Error:      "#fb4934",
	Info:       "#83a598",
}

var Monochrome = Theme{
	Name:       "monochrome",
	Background: "",       // terminal default
	Foreground: "#ffffff",
	Subtle:     "#888888",
	Border:     "#555555",
	Accent:     "#ffffff",
	Accent2:    "#bbbbbb",
	Success:    "#ffffff",
	Warning:    "#ffffff",
	Error:      "#ffffff",
	Info:       "#ffffff",
}

// All returns themes in display order.
func All() []Theme {
	return []Theme{Catppuccin, TokyoNight, Gruvbox, Monochrome}
}

func ByName(name string) Theme {
	for _, t := range All() {
		if t.Name == name {
			return t
		}
	}
	return Catppuccin
}

// Convenience styles built from a theme.
type Styles struct {
	Title      lipgloss.Style
	Subtle     lipgloss.Style
	Accent     lipgloss.Style
	Accent2    lipgloss.Style
	Success    lipgloss.Style
	Warning    lipgloss.Style
	Error      lipgloss.Style
	Info       lipgloss.Style
	Border     lipgloss.Style
	Box        lipgloss.Style
	BoxTitle   lipgloss.Style
	Footer     lipgloss.Style
	Selected   lipgloss.Style
	InputField lipgloss.Style
	InputFocus lipgloss.Style
}

func (t Theme) Styles() Styles {
	return Styles{
		Title:    lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		Subtle:   lipgloss.NewStyle().Foreground(t.Subtle),
		Accent:   lipgloss.NewStyle().Foreground(t.Accent),
		Accent2:  lipgloss.NewStyle().Foreground(t.Accent2),
		Success:  lipgloss.NewStyle().Foreground(t.Success),
		Warning:  lipgloss.NewStyle().Foreground(t.Warning),
		Error:    lipgloss.NewStyle().Foreground(t.Error),
		Info:     lipgloss.NewStyle().Foreground(t.Info),
		Border:   lipgloss.NewStyle().Foreground(t.Border),
		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),
		BoxTitle: lipgloss.NewStyle().
			Foreground(t.Accent).
			Bold(true).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(t.Subtle).
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(t.Background).
			Background(t.Accent).
			Bold(true),
		InputField: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(t.Border),
		InputFocus: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(t.Accent),
	}
}
