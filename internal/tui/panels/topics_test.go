package panels

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/D1ssolve/franzctl/internal/domain"
)

func TestTopicsPanelSelectionEmitsMessage(t *testing.T) {
	t.Parallel()
	panel := NewTopicsPanel(40, 10)
	panel.SetTopics([]domain.Topic{{Name: "a"}, {Name: "b"}})
	panel.SetFocused(true)

	panel, cmd := panel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if selected := panel.Selected(); selected == nil || selected.Name != "b" {
		t.Fatalf("selected = %#v", selected)
	}
	if cmd == nil {
		t.Fatal("expected selection command")
	}
	msg, ok := cmd().(TopicSelectedMsg)
	if !ok || msg.Topic != "b" {
		t.Fatalf("message = %#v", cmd())
	}
}
