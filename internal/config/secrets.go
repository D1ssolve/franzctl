package config

import "errors"

const secretService = "franzctl"

var ErrSecretNotFound = errors.New("credentials not found")

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c Credentials) Empty() bool {
	return c.Username == "" && c.Password == ""
}

type SecretStore interface {
	Get(profileID string) (Credentials, error)
	Set(profileID string, credentials Credentials) error
	Delete(profileID string) error
}

func NewSecretStore(profileStore *ProfileStore) SecretStore {
	return newPlatformSecretStore(profileStore)
}
