package tui

import "github.com/charmbracelet/lipgloss"

const ColorPrimary = lipgloss.Color("#7C3AED")

type Styles struct {
	Header lipgloss.Style
	Footer lipgloss.Style
	Modal  lipgloss.Style
	Muted  lipgloss.Style
	Error  lipgloss.Style
}

func NewStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary),
		Footer: lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")),
		Modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2),
		Muted: lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")),
		Error: lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")),
	}
}
