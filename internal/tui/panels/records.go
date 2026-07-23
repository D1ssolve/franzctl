package panels

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/D1ssolve/franzctl/internal/domain"
)

type RecordsPanel struct {
	topic   string
	records []domain.Record
	index   int
	focused bool
	width   int
	height  int
}

func NewRecordsPanel(width, height int) RecordsPanel {
	return RecordsPanel{width: width, height: height}
}

func (p *RecordsPanel) SetRecords(topic string, records []domain.Record) {
	p.topic = topic
	p.records = append([]domain.Record(nil), records...)
	p.index = 0
}

func (p *RecordsPanel) SetSize(width, height int) {
	p.width, p.height = width, height
}

func (p *RecordsPanel) SetFocused(focused bool) {
	p.focused = focused
}

func (p RecordsPanel) Selected() *domain.Record {
	if p.index < 0 || p.index >= len(p.records) {
		return nil
	}
	item := p.records[p.index]
	return &item
}

func (p RecordsPanel) Update(msg tea.Msg) (RecordsPanel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok || !p.focused {
		return p, nil
	}

	previous := p.index
	switch key.String() {
	case "j", "down":
		if p.index < len(p.records)-1 {
			p.index++
		}
	case "k", "up":
		if p.index > 0 {
			p.index--
		}
	case "h", "esc":
		return p, func() tea.Msg { return FocusTopicsMsg{} }
	case "p":
		if p.topic != "" {
			topic := p.topic
			return p, func() tea.Msg { return OpenProduceMsg{Topic: topic} }
		}
	}
	if previous != p.index {
		record := p.Selected()
		return p, func() tea.Msg { return RecordSelectedMsg{Record: record} }
	}
	return p, nil
}

func (p RecordsPanel) View() string {
	innerWidth, innerHeight := innerSize(p.width, p.height)
	titleText := "[2] Records"
	if p.topic != "" {
		titleText = fmt.Sprintf("[2] Records — %s  [%d]", p.topic, len(p.records))
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(titleText)
	lines := []string{title}

	if p.topic == "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render("Select a topic."))
	} else if len(p.records) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render("No records in the current snapshot."))
	} else {
		visible := max(1, innerHeight-1)
		start := 0
		if p.index >= visible {
			start = p.index - visible + 1
		}
		for i := start; i < len(p.records) && len(lines) < innerHeight; i++ {
			record := p.records[i]
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == p.index {
				prefix = "› "
				style = style.Bold(true).Foreground(ColorPrimary)
			}
			preview := strings.ReplaceAll(string(record.Value), "\n", " ")
			if len(preview) > 42 {
				preview = preview[:41] + "…"
			}
			line := fmt.Sprintf("%s%d:%d  %s", prefix, record.Partition, record.Offset, preview)
			lines = append(lines, style.MaxWidth(innerWidth).Render(line))
		}
	}

	return borderStyle(p.focused).Width(innerWidth).Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}
