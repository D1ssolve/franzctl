package config

import (
	"fmt"
	"strings"
)

type KeyValues map[string]string

func (k *KeyValues) String() string {
	if k == nil || *k == nil {
		return ""
	}
	parts := make([]string, 0, len(*k))
	for key, value := range *k {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ",")
}

func (k *KeyValues) Set(value string) error {
	key, val, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("expected key=value, got %q", value)
	}
	if *k == nil {
		*k = make(map[string]string)
	}
	(*k)[strings.TrimSpace(key)] = val
	return nil
}
