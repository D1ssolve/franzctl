//go:build linux

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type linuxSecretStore struct{}

func newPlatformSecretStore(_ *ProfileStore) SecretStore {
	return linuxSecretStore{}
}

func (linuxSecretStore) Get(profileID string) (Credentials, error) {
	path, err := exec.LookPath("secret-tool")
	if err != nil {
		return Credentials{}, linuxKeyringUnavailable()
	}
	output, err := exec.Command(path, "lookup", "service", secretService, "profile", profileID).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return Credentials{}, ErrSecretNotFound
		}
		return Credentials{}, fmt.Errorf("read credentials from Secret Service: %w", err)
	}
	var credentials Credentials
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(output))), &credentials); err != nil {
		return Credentials{}, fmt.Errorf("decode credentials from Secret Service: %w", err)
	}
	return credentials, nil
}

func (linuxSecretStore) Set(profileID string, credentials Credentials) error {
	path, err := exec.LookPath("secret-tool")
	if err != nil {
		return linuxKeyringUnavailable()
	}
	payload, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	cmd := exec.Command(
		path,
		"store",
		"--label=franzctl connection "+profileID,
		"service", secretService,
		"profile", profileID,
	)
	cmd.Stdin = strings.NewReader(string(payload))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("store credentials in Secret Service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (linuxSecretStore) Delete(profileID string) error {
	path, err := exec.LookPath("secret-tool")
	if err != nil {
		return linuxKeyringUnavailable()
	}
	cmd := exec.Command(path, "clear", "service", secretService, "profile", profileID)
	if output, err := cmd.CombinedOutput(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("delete credentials from Secret Service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func linuxKeyringUnavailable() error {
	return errors.New("secure credential storage requires secret-tool (install libsecret-tools); credentials were not stored")
}
