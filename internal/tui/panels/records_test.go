package panels

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/D1ssolve/franzctl/internal/domain"
)

func TestRecordsPanelProduceUsesCurrentTopic(t *testing.T) {
	t.Parallel()
	panel := NewRecordsPanel(60, 10)
	panel.SetRecords("events", []domain.Record{{Offset: 1}})
	panel.SetFocused(true)

	_, cmd := panel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("expected produce command")
	}
	msg, ok := cmd().(OpenProduceMsg)
	if !ok || msg.Topic != "events" {
		t.Fatalf("message = %#v", cmd())
	}
}
