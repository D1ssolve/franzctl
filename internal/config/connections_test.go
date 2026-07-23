package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type memorySecretStore struct {
	values map[string]Credentials
}

func (s *memorySecretStore) Get(profileID string) (Credentials, error) {
	value, ok := s.values[profileID]
	if !ok {
		return Credentials{}, ErrSecretNotFound
	}
	return value, nil
}

func (s *memorySecretStore) Set(profileID string, credentials Credentials) error {
	s.values[profileID] = credentials
	return nil
}

func (s *memorySecretStore) Delete(profileID string) error {
	delete(s.values, profileID)
	return nil
}

func TestProfileStoreRoundTripAndCurrentSelection(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "franzctl", "connections.json")
	store := NewProfileStore(path)

	first := testProfile(t, "local", "localhost:9092")
	second := testProfile(t, "staging", "kafka.example.test:9093")
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Use(second.Name); err != nil {
		t.Fatal(err)
	}

	current, err := store.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != second.ID {
		t.Fatalf("current connection = %q, want %q", current.Name, second.Name)
	}
	profiles, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("profile count = %d, want 2", len(profiles))
	}
	if profiles[0].ID != second.ID {
		t.Fatalf("most recently used profile = %q, want %q", profiles[0].Name, second.Name)
	}

	removed, err := store.Delete(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if removed.ID != second.ID {
		t.Fatalf("removed connection = %q, want %q", removed.Name, second.Name)
	}
	current, err = store.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != first.ID {
		t.Fatalf("fallback current connection = %q, want %q", current.Name, first.Name)
	}
}

func TestProfileFileNeverContainsCredentials(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "connections.json")
	store := NewProfileStore(path)
	profile := testProfile(t, "production", "broker.example.test:9093")
	profile.CredentialsStored = true
	if err := store.Save(profile); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"alice@example.test", "correct horse battery staple"} {
		if strings.Contains(string(content), secret) {
			t.Fatalf("connections file contains secret %q", secret)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Fatalf("connections file permissions = %o, want 600", permissions)
	}
}

func TestClientFromProfileLoadsCredentials(t *testing.T) {
	t.Parallel()
	store := NewProfileStore(filepath.Join(t.TempDir(), "connections.json"))
	profile := testProfile(t, "cloud", "broker.example.test:9093")
	profile.SASLMechanism = "scram-sha-512"
	profile.TLSEnabled = true
	profile.CredentialsStored = true
	secrets := &memorySecretStore{values: map[string]Credentials{
		profile.ID: {Username: "alice", Password: "secret"},
	}}

	client, err := ClientFromProfile(store, secrets, profile)
	if err != nil {
		t.Fatal(err)
	}
	if client.SASLUsername != "alice" || client.SASLPassword != "secret" {
		t.Fatalf("loaded credentials = %q/%q", client.SASLUsername, client.SASLPassword)
	}
	if !client.TLSEnabled || client.Connection != "cloud" {
		t.Fatalf("loaded client = %#v", client)
	}
}

func TestClientFromProfileRejectsMissingCredentials(t *testing.T) {
	t.Parallel()
	store := NewProfileStore(filepath.Join(t.TempDir(), "connections.json"))
	profile := testProfile(t, "cloud", "broker.example.test:9093")
	profile.CredentialsStored = true
	secrets := &memorySecretStore{values: map[string]Credentials{}}

	_, err := ClientFromProfile(store, secrets, profile)
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("error = %v, want missing credentials error", err)
	}
}

func TestResolveUnknownProfile(t *testing.T) {
	t.Parallel()
	store := NewProfileStore(filepath.Join(t.TempDir(), "connections.json"))
	_, err := store.Resolve("missing")
	if !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("error = %v, want ErrConnectionNotFound", err)
	}
}

func TestExplicitBrokerDoesNotInheritActiveProfile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	unsetEnv(t, "FRANZCTL_CONNECTION")
	unsetEnv(t, "FRANZCTL_BROKERS")
	unsetEnv(t, "FRANZCTL_TLS")
	unsetEnv(t, "FRANZCTL_SASL_MECHANISM")
	unsetEnv(t, "FRANZCTL_SASL_USERNAME")
	unsetEnv(t, "FRANZCTL_SASL_PASSWORD")

	store, err := DefaultProfileStore()
	if err != nil {
		t.Fatal(err)
	}
	profile := testProfile(t, "production", "production.example.test:9093")
	profile.TLSEnabled = true
	if err := store.Save(profile); err != nil {
		t.Fatal(err)
	}

	client, err := LoadForArgs([]string{"--broker", "other.example.test:9092"})
	if err != nil {
		t.Fatal(err)
	}
	if client.Connection != "" || client.TLSEnabled {
		t.Fatalf("explicit broker inherited active profile: %#v", client)
	}
	if len(client.Brokers) != 0 {
		t.Fatalf("brokers = %#v, want flag parser to populate them", client.Brokers)
	}
}

func TestExplicitConnectionLoadsSavedProfile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	unsetEnv(t, "FRANZCTL_CONNECTION")
	unsetEnv(t, "FRANZCTL_BROKERS")
	unsetEnv(t, "FRANZCTL_TLS")

	store, err := DefaultProfileStore()
	if err != nil {
		t.Fatal(err)
	}
	profile := testProfile(t, "staging", "staging.example.test:9093")
	profile.TLSEnabled = true
	if err := store.Save(profile); err != nil {
		t.Fatal(err)
	}

	client, err := LoadForArgs([]string{"--connection", "staging"})
	if err != nil {
		t.Fatal(err)
	}
	if client.Connection != "staging" || !client.TLSEnabled {
		t.Fatalf("loaded client = %#v", client)
	}
}

func testProfile(t *testing.T, name, broker string) ConnectionProfile {
	t.Helper()
	client := Client{
		Brokers:        Strings{broker},
		ClientID:       "franzctl",
		RequestTimeout: 15 * time.Second,
	}
	profile, err := NewConnectionProfile(name, client)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	value, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, value)
			return
		}
		_ = os.Unsetenv(key)
	})
}
