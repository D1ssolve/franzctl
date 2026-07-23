package panels

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/D1ssolve/franzctl/internal/domain"
)

type TopicsPanel struct {
	topics  []domain.Topic
	index   int
	focused bool
	width   int
	height  int
}

func NewTopicsPanel(width, height int) TopicsPanel {
	return TopicsPanel{width: width, height: height}
}

func (p *TopicsPanel) SetTopics(topics []domain.Topic) {
	p.topics = append([]domain.Topic(nil), topics...)
	if p.index >= len(p.topics) {
		p.index = max(0, len(p.topics)-1)
	}
}

func (p *TopicsPanel) SetSize(width, height int) {
	p.width, p.height = width, height
}

func (p *TopicsPanel) SetFocused(focused bool) {
	p.focused = focused
}

func (p TopicsPanel) Selected() *domain.Topic {
	if p.index < 0 || p.index >= len(p.topics) {
		return nil
	}
	item := p.topics[p.index]
	return &item
}

func (p TopicsPanel) Update(msg tea.Msg) (TopicsPanel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok || !p.focused {
		return p, nil
	}

	previous := p.index
	switch key.String() {
	case "j", "down":
		if p.index < len(p.topics)-1 {
			p.index++
		}
	case "k", "up":
		if p.index > 0 {
			p.index--
		}
	case "enter", "l":
		if p.Selected() != nil {
			return p, func() tea.Msg { return FocusRecordsMsg{} }
		}
	case "n":
		return p, func() tea.Msg { return OpenCreateTopicMsg{} }
	}
	if previous != p.index {
		name := p.topics[p.index].Name
		return p, func() tea.Msg { return TopicSelectedMsg{Topic: name} }
	}
	return p, nil
}

func (p TopicsPanel) View() string {
	innerWidth, innerHeight := innerSize(p.width, p.height)
	title := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).
		Render(fmt.Sprintf("[1] Topics  [%d]", len(p.topics)))

	lines := []string{title}
	if len(p.topics) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render("No topics. Press [n] to create one."))
	} else {
		visible := max(1, innerHeight-1)
		start := 0
		if p.index >= visible {
			start = p.index - visible + 1
		}
		for i := start; i < len(p.topics) && len(lines) < innerHeight; i++ {
			topic := p.topics[i]
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == p.index {
				prefix = "› "
				style = style.Bold(true).Foreground(ColorPrimary)
			} else if topic.Internal {
				style = style.Foreground(ColorMuted)
			}
			line := fmt.Sprintf("%s%s  p:%d r:%d", prefix, topic.Name, topic.Partitions, topic.ReplicationFactor)
			lines = append(lines, style.MaxWidth(innerWidth).Render(line))
		}
	}

	return borderStyle(p.focused).Width(innerWidth).Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}
