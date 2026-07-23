package panels

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type OutputPanel struct {
	lines  []string
	width  int
	height int
}

func NewOutputPanel(width, height int) OutputPanel {
	return OutputPanel{width: width, height: height}
}

func (p *OutputPanel) SetSize(width, height int) {
	p.width, p.height = width, height
}

func (p *OutputPanel) Append(line string) {
	p.lines = append(p.lines, line)
	if len(p.lines) > 500 {
		p.lines = append([]string(nil), p.lines[len(p.lines)-500:]...)
	}
}

func (p OutputPanel) View() string {
	innerWidth, innerHeight := innerSize(p.width, p.height)
	title := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("[0] Output")
	lines := []string{title}
	visible := max(0, innerHeight-1)
	start := max(0, len(p.lines)-visible)
	for _, line := range p.lines[start:] {
		lines = append(lines, lipgloss.NewStyle().MaxWidth(innerWidth).Render(line))
	}
	return borderStyle(false).Width(innerWidth).Height(innerHeight).Render(strings.Join(lines, "\n"))
}
