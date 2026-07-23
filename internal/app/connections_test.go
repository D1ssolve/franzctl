package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectionCommandsManageUnauthenticatedProfiles(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("FRANZCTL_CONNECTION", "")
	t.Setenv("FRANZCTL_BROKERS", "")

	application := New("dev", "none", "unknown")
	var stdout, stderr bytes.Buffer

	code := application.Run(
		[]string{"connection", "add", "--name", "local", "--broker", "localhost:9092"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("add exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Saved and selected connection "local"`) {
		t.Fatalf("add stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := application.Run(
		[]string{"connection", "list"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	); code != 0 {
		t.Fatalf("list exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "* local") {
		t.Fatalf("list stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := application.Run(
		[]string{"connection", "remove", "local"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	); code != 0 {
		t.Fatalf("remove exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Removed connection "local"`) {
		t.Fatalf("remove stdout = %q", stdout.String())
	}

	connectionsPath := filepath.Join(configDir, "franzctl", "connections.json")
	content, err := os.ReadFile(connectionsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "local") {
		t.Fatalf("removed profile remains in %s: %s", connectionsPath, content)
	}
}
