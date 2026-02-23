package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette used by the TUI.
type Theme struct {
	Accent    lipgloss.Color
	AccentDim lipgloss.Color
	Text      lipgloss.Color
	Subtle    lipgloss.Color
	Error     lipgloss.Color
}

// Styles contains lipgloss styles derived from a Theme.
type Styles struct {
	Header      lipgloss.Style
	Row         lipgloss.Style
	SelectedRow lipgloss.Style
	Status      lipgloss.Style
	StatusError lipgloss.Style
	Help        lipgloss.Style
	Empty       lipgloss.Style
	Confirm     lipgloss.Style
}

// DefaultTheme returns the retro-inspired default palette.
func DefaultTheme() Theme {
	return Theme{
		Accent:    lipgloss.Color("#00FFB2"),
		AccentDim: lipgloss.Color("#0B4E3A"),
		Text:      lipgloss.Color("#E6FFF6"),
		Subtle:    lipgloss.Color("#6B8D83"),
		Error:     lipgloss.Color("#FF5F5F"),
	}
}

// NewStyles builds UI styles from a theme.
func NewStyles(theme Theme) Styles {
	base := lipgloss.NewStyle().Foreground(theme.Text)

	return Styles{
		Header:      base.Copy().Foreground(theme.Accent).Bold(true),
		Row:         base.Copy(),
		SelectedRow: lipgloss.NewStyle().Foreground(lipgloss.Color("#00140F")).Background(theme.Accent).Bold(true),
		Status:      base.Copy().Foreground(theme.Subtle),
		StatusError: base.Copy().Foreground(theme.Error).Bold(true),
		Help:        base.Copy().Foreground(theme.Subtle),
		Empty:       base.Copy().Foreground(theme.Subtle),
		Confirm:     base.Copy().Foreground(theme.Accent).Bold(true),
	}
}
