package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/D1ssolve/franzctl/internal/config"
)

func TestConnectionSelectorSelectsSavedProfile(t *testing.T) {
	t.Parallel()
	profile, err := config.NewConnectionProfile("staging", config.Client{
		Brokers:        config.Strings{"staging.example.test:9093"},
		ClientID:       "franzctl",
		RequestTimeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	model := NewConnectionSelector([]config.ConnectionProfile{profile}, profile.ID)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(ConnectionSelector)
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	selection, err := model.Selection()
	if err != nil {
		t.Fatal(err)
	}
	if selection.Profile.ID != profile.ID || selection.Created {
		t.Fatalf("selection = %#v", selection)
	}
}

func TestConnectionSelectorBuildsSecureProfile(t *testing.T) {
	t.Parallel()
	model := NewConnectionSelector(nil, "")
	model.inputs[0].SetValue("production")
	model.inputs[1].SetValue("one.example.test:9093,two.example.test:9093")
	model.inputs[2].SetValue("true")
	model.inputs[3].SetValue("scram-sha-512")
	model.inputs[4].SetValue("alice")
	model.inputs[5].SetValue("secret")

	selection, err := model.buildConnection()
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Created || !selection.Profile.CredentialsStored {
		t.Fatalf("selection = %#v", selection)
	}
	if len(selection.Profile.Brokers) != 2 {
		t.Fatalf("brokers = %#v", selection.Profile.Brokers)
	}
	if selection.Credentials.Username != "alice" || selection.Credentials.Password != "secret" {
		t.Fatalf("credentials = %#v", selection.Credentials)
	}
}

func TestConnectionSelectorRejectsSASLWithoutPassword(t *testing.T) {
	t.Parallel()
	model := NewConnectionSelector(nil, "")
	model.inputs[0].SetValue("broken")
	model.inputs[1].SetValue("broker.example.test:9093")
	model.inputs[2].SetValue("true")
	model.inputs[3].SetValue("plain")
	model.inputs[4].SetValue("alice")

	if _, err := model.buildConnection(); err == nil {
		t.Fatal("expected password validation error")
	}
}
