package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpAndVersion(t *testing.T) {
	t.Parallel()
	application := New("1.2.3", "abc123", "2026-01-01")

	var stdout, stderr bytes.Buffer
	if code := application.Run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "topic create") {
		t.Fatalf("help output = %q", stdout.String())
	}

	stdout.Reset()
	if code := application.Run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("version exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "1.2.3") {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	application := New("dev", "none", "unknown")
	var stdout, stderr bytes.Buffer
	if code := application.Run([]string{"wat"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
