package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/D1ssolve/franzctl/internal/domain"
)

type fakeManager struct {
	topics  []domain.Topic
	records []domain.Record
	created []domain.CreateTopicParams
	sent    []domain.ProduceParams
}

func (f *fakeManager) ListTopics(context.Context) ([]domain.Topic, error) {
	return append([]domain.Topic(nil), f.topics...), nil
}

func (f *fakeManager) ReadRecords(context.Context, string, int) ([]domain.Record, error) {
	return append([]domain.Record(nil), f.records...), nil
}

func (f *fakeManager) CreateTopic(_ context.Context, params domain.CreateTopicParams) error {
	f.created = append(f.created, params)
	return nil
}

func (f *fakeManager) Produce(_ context.Context, params domain.ProduceParams) (domain.Record, error) {
	f.sent = append(f.sent, params)
	return domain.Record{Topic: params.Topic, Partition: 0, Offset: 4, Value: params.Value}, nil
}

func TestModelLoadsTopicsAndRecords(t *testing.T) {
	t.Parallel()
	manager := &fakeManager{
		topics: []domain.Topic{{Name: "events", Partitions: 3, ReplicationFactor: 1}},
		records: []domain.Record{{
			Topic: "events", Partition: 0, Offset: 12,
			Timestamp: time.Unix(100, 0), Value: []byte(`{"type":"created"}`),
		}},
	}
	model, err := New(manager, []string{"localhost:9092"}, "dev")
	if err != nil {
		t.Fatal(err)
	}

	msg := model.Init()()
	updated, cmd := model.Update(msg)
	model = updated.(Model)
	if selected := model.topics.Selected(); selected == nil || selected.Name != "events" {
		t.Fatalf("selected topic = %#v", selected)
	}
	if cmd == nil {
		t.Fatal("expected records command")
	}

	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if selected := model.records.Selected(); selected == nil || selected.Offset != 12 {
		t.Fatalf("selected record = %#v", selected)
	}
}

func TestTopicDialogOpensFromTopicsPanel(t *testing.T) {
	t.Parallel()
	manager := &fakeManager{}
	model, err := New(manager, nil, "dev")
	if err != nil {
		t.Fatal(err)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected open-dialog command")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if model.dialog == nil || model.dialog.kind != dialogCreateTopic {
		t.Fatalf("dialog = %#v", model.dialog)
	}
}

func TestNewRejectsNilManager(t *testing.T) {
	t.Parallel()
	if _, err := New(nil, nil, "dev"); err == nil {
		t.Fatal("expected error")
	}
}
