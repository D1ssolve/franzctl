//go:build darwin

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type darwinSecretStore struct{}

func newPlatformSecretStore(_ *ProfileStore) SecretStore {
	return darwinSecretStore{}
}

func (darwinSecretStore) Get(profileID string) (Credentials, error) {
	output, err := exec.Command(
		"/usr/bin/security",
		"find-generic-password",
		"-s", secretService,
		"-a", profileID,
		"-w",
	).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			return Credentials{}, ErrSecretNotFound
		}
		return Credentials{}, fmt.Errorf("read credentials from Keychain: %w", err)
	}
	var credentials Credentials
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(output))), &credentials); err != nil {
		return Credentials{}, fmt.Errorf("decode credentials from Keychain: %w", err)
	}
	return credentials, nil
}

func (darwinSecretStore) Set(profileID string, credentials Credentials) error {
	payload, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	output, err := exec.Command(
		"/usr/bin/security",
		"add-generic-password",
		"-U",
		"-s", secretService,
		"-a", profileID,
		"-w", string(payload),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("store credentials in Keychain: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (darwinSecretStore) Delete(profileID string) error {
	output, err := exec.Command(
		"/usr/bin/security",
		"delete-generic-password",
		"-s", secretService,
		"-a", profileID,
	).CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			return nil
		}
		return fmt.Errorf("delete credentials from Keychain: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
