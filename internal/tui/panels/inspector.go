package panels

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/D1ssolve/franzctl/internal/domain"
	recordjson "github.com/D1ssolve/franzctl/internal/record"
)

type InspectorPanel struct {
	record *domain.Record
	width  int
	height int
}

func NewInspectorPanel(width, height int) InspectorPanel {
	return InspectorPanel{width: width, height: height}
}

func (p *InspectorPanel) SetSize(width, height int) {
	p.width, p.height = width, height
}

func (p *InspectorPanel) SetRecord(record *domain.Record) {
	if record == nil {
		p.record = nil
		return
	}
	copy := *record
	p.record = &copy
}

func (p InspectorPanel) View() string {
	innerWidth, innerHeight := innerSize(p.width, p.height)
	title := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("[3] Inspector")
	lines := []string{title}
	if p.record == nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render("Select a record."))
	} else {
		record := p.record
		lines = append(lines,
			fmt.Sprintf("partition  %d", record.Partition),
			fmt.Sprintf("offset     %d", record.Offset),
			fmt.Sprintf("time       %s", record.Timestamp.Format("15:04:05.000")),
			fmt.Sprintf("key        %v", recordjson.Value(record.Key)),
			"",
			lipgloss.NewStyle().Bold(true).Render("value"),
		)
		value := recordjson.Value(record.Value)
		if formatted, err := json.MarshalIndent(value, "", "  "); err == nil {
			lines = append(lines, strings.Split(string(formatted), "\n")...)
		} else {
			lines = append(lines, fmt.Sprint(value))
		}
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	return borderStyle(false).Width(innerWidth).Height(innerHeight).
		Render(lipgloss.NewStyle().MaxWidth(innerWidth).Render(strings.Join(lines, "\n")))
}
