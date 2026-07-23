//go:build windows

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsSecretStore struct {
	dir string
}

func newPlatformSecretStore(profileStore *ProfileStore) SecretStore {
	return windowsSecretStore{dir: filepath.Join(filepath.Dir(profileStore.Path()), "secrets")}
}

func (s windowsSecretStore) Get(profileID string) (Credentials, error) {
	ciphertext, err := os.ReadFile(s.path(profileID))
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, ErrSecretNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("read protected credentials: %w", err)
	}
	payload, err := unprotectCurrentUser(ciphertext)
	if err != nil {
		return Credentials{}, fmt.Errorf("decrypt credentials with DPAPI: %w", err)
	}
	var credentials Credentials
	if err := json.Unmarshal(payload, &credentials); err != nil {
		return Credentials{}, fmt.Errorf("decode credentials: %w", err)
	}
	return credentials, nil
}

func (s windowsSecretStore) Set(profileID string, credentials Credentials) error {
	payload, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	ciphertext, err := protectCurrentUser(payload)
	if err != nil {
		return fmt.Errorf("encrypt credentials with DPAPI: %w", err)
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create protected credentials directory: %w", err)
	}
	if err := os.WriteFile(s.path(profileID), ciphertext, 0o600); err != nil {
		return fmt.Errorf("write protected credentials: %w", err)
	}
	return nil
}

func (s windowsSecretStore) Delete(profileID string) error {
	if err := os.Remove(s.path(profileID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete protected credentials: %w", err)
	}
	return nil
}

func (s windowsSecretStore) path(profileID string) string {
	return filepath.Join(s.dir, profileID+".dpapi")
}

func protectCurrentUser(plaintext []byte) ([]byte, error) {
	input := dataBlob(plaintext)
	var output windows.DataBlob
	if err := windows.CryptProtectData(
		&input,
		nil,
		nil,
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&output,
	); err != nil {
		return nil, err
	}
	return copyAndFreeBlob(output), nil
}

func unprotectCurrentUser(ciphertext []byte) ([]byte, error) {
	input := dataBlob(ciphertext)
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(
		&input,
		nil,
		nil,
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&output,
	); err != nil {
		return nil, err
	}
	return copyAndFreeBlob(output), nil
}

func dataBlob(value []byte) windows.DataBlob {
	if len(value) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(value)), Data: &value[0]}
}

func copyAndFreeBlob(blob windows.DataBlob) []byte {
	if blob.Data == nil || blob.Size == 0 {
		return nil
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(blob.Data)))
	return append([]byte(nil), unsafe.Slice(blob.Data, int(blob.Size))...)
}
