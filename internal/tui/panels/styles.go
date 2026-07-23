package panels

import "github.com/charmbracelet/lipgloss"

const (
	ColorPrimary  = lipgloss.Color("#7C3AED")
	ColorMuted    = lipgloss.Color("#6B7280")
	ColorGood     = lipgloss.Color("#34D399")
	ColorWarning  = lipgloss.Color("#FBBF24")
	ColorInactive = lipgloss.Color("#374151")
)

func borderStyle(focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	if focused {
		return style.BorderForeground(ColorPrimary)
	}
	return style.BorderForeground(ColorInactive)
}

func innerSize(width, height int) (int, int) {
	return max(0, width-2), max(0, height-2)
}
