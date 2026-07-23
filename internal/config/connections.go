package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const connectionsFileVersion = 1

var ErrConnectionNotFound = errors.New("connection not found")

type ConnectionProfile struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Brokers           []string  `json:"brokers"`
	ClientID          string    `json:"client_id,omitempty"`
	RequestTimeout    string    `json:"request_timeout,omitempty"`
	TLSEnabled        bool      `json:"tls_enabled,omitempty"`
	TLSInsecure       bool      `json:"tls_insecure,omitempty"`
	TLSCA             string    `json:"tls_ca,omitempty"`
	TLSCert           string    `json:"tls_cert,omitempty"`
	TLSKey            string    `json:"tls_key,omitempty"`
	SASLMechanism     string    `json:"sasl_mechanism,omitempty"`
	CredentialsStored bool      `json:"credentials_stored,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	LastUsedAt        time.Time `json:"last_used_at"`
}

func NewConnectionProfile(name string, client Client) (ConnectionProfile, error) {
	id, err := randomID()
	if err != nil {
		return ConnectionProfile{}, err
	}
	now := time.Now().UTC()
	profile := ConnectionProfile{
		ID:             id,
		Name:           strings.TrimSpace(name),
		Brokers:        append([]string(nil), client.Brokers...),
		ClientID:       client.ClientID,
		RequestTimeout: client.RequestTimeout.String(),
		TLSEnabled:     client.TLSEnabled,
		TLSInsecure:    client.TLSInsecure,
		TLSCA:          client.TLSCA,
		TLSCert:        client.TLSCert,
		TLSKey:         client.TLSKey,
		SASLMechanism:  strings.ToLower(client.SASLMechanism),
		CreatedAt:      now,
		LastUsedAt:     now,
	}
	if err := profile.Validate(); err != nil {
		return ConnectionProfile{}, err
	}
	return profile, nil
}

func (p ConnectionProfile) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("connection id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("connection name is required")
	}
	if len(p.Brokers) == 0 {
		return errors.New("at least one broker is required")
	}
	for _, broker := range p.Brokers {
		if strings.TrimSpace(broker) == "" {
			return errors.New("broker address must not be empty")
		}
	}
	if p.RequestTimeout != "" {
		timeout, err := time.ParseDuration(p.RequestTimeout)
		if err != nil || timeout <= 0 {
			return fmt.Errorf("invalid request timeout %q", p.RequestTimeout)
		}
	}
	return nil
}

func (p ConnectionProfile) Client(credentials Credentials) (Client, error) {
	timeout := 15 * time.Second
	if p.RequestTimeout != "" {
		parsed, err := time.ParseDuration(p.RequestTimeout)
		if err != nil {
			return Client{}, fmt.Errorf("connection %q: invalid request timeout: %w", p.Name, err)
		}
		timeout = parsed
	}
	client := Client{
		Connection:     p.Name,
		Brokers:        append(Strings(nil), p.Brokers...),
		ClientID:       p.ClientID,
		RequestTimeout: timeout,
		TLSEnabled:     p.TLSEnabled,
		TLSInsecure:    p.TLSInsecure,
		TLSCA:          p.TLSCA,
		TLSCert:        p.TLSCert,
		TLSKey:         p.TLSKey,
		SASLMechanism:  p.SASLMechanism,
		SASLUsername:   credentials.Username,
		SASLPassword:   credentials.Password,
	}
	if client.ClientID == "" {
		client.ClientID = "franzctl"
	}
	return client, client.Validate()
}

type connectionsFile struct {
	Version  int                 `json:"version"`
	Current  string              `json:"current,omitempty"`
	Profiles []ConnectionProfile `json:"profiles"`
}

type ProfileStore struct {
	path string
	mu   sync.Mutex
}

func DefaultProfileStore() (*ProfileStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}
	return NewProfileStore(filepath.Join(dir, "franzctl", "connections.json")), nil
}

func NewProfileStore(path string) *ProfileStore {
	return &ProfileStore{path: path}
}

func (s *ProfileStore) Path() string {
	return s.path
}

func (s *ProfileStore) List() ([]ConnectionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	profiles := append([]ConnectionProfile(nil), data.Profiles...)
	sort.SliceStable(profiles, func(i, j int) bool {
		return profiles[i].LastUsedAt.After(profiles[j].LastUsedAt)
	})
	return profiles, nil
}

func (s *ProfileStore) Resolve(reference string) (ConnectionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return ConnectionProfile{}, err
	}
	return resolveProfile(data, reference)
}

func (s *ProfileStore) Current() (ConnectionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return ConnectionProfile{}, err
	}
	if data.Current == "" {
		if len(data.Profiles) == 0 {
			return ConnectionProfile{}, ErrConnectionNotFound
		}
		return mostRecentlyUsed(data.Profiles), nil
	}
	return resolveProfile(data, data.Current)
}

func (s *ProfileStore) Save(profile ConnectionProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	for _, existing := range data.Profiles {
		if existing.ID != profile.ID && strings.EqualFold(existing.Name, profile.Name) {
			return fmt.Errorf("connection %q already exists", profile.Name)
		}
	}
	found := false
	for i := range data.Profiles {
		if data.Profiles[i].ID == profile.ID {
			data.Profiles[i] = profile
			found = true
			break
		}
	}
	if !found {
		data.Profiles = append(data.Profiles, profile)
	}
	if data.Current == "" {
		data.Current = profile.ID
	}
	return s.write(data)
}

func (s *ProfileStore) Use(reference string) (ConnectionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return ConnectionProfile{}, err
	}
	profile, err := resolveProfile(data, reference)
	if err != nil {
		return ConnectionProfile{}, err
	}
	now := time.Now().UTC()
	for i := range data.Profiles {
		if data.Profiles[i].ID == profile.ID {
			data.Profiles[i].LastUsedAt = now
			profile = data.Profiles[i]
			break
		}
	}
	data.Current = profile.ID
	if err := s.write(data); err != nil {
		return ConnectionProfile{}, err
	}
	return profile, nil
}

func (s *ProfileStore) Delete(reference string) (ConnectionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return ConnectionProfile{}, err
	}
	profile, err := resolveProfile(data, reference)
	if err != nil {
		return ConnectionProfile{}, err
	}
	filtered := data.Profiles[:0]
	for _, candidate := range data.Profiles {
		if candidate.ID != profile.ID {
			filtered = append(filtered, candidate)
		}
	}
	data.Profiles = filtered
	if data.Current == profile.ID {
		data.Current = ""
		if len(data.Profiles) > 0 {
			data.Current = mostRecentlyUsed(data.Profiles).ID
		}
	}
	if err := s.write(data); err != nil {
		return ConnectionProfile{}, err
	}
	return profile, nil
}

func (s *ProfileStore) read() (connectionsFile, error) {
	input, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return connectionsFile{Version: connectionsFileVersion, Profiles: []ConnectionProfile{}}, nil
	}
	if err != nil {
		return connectionsFile{}, fmt.Errorf("read connections: %w", err)
	}
	var data connectionsFile
	if err := json.Unmarshal(input, &data); err != nil {
		return connectionsFile{}, fmt.Errorf("decode connections: %w", err)
	}
	if data.Version != connectionsFileVersion {
		return connectionsFile{}, fmt.Errorf("unsupported connections file version %d", data.Version)
	}
	for _, profile := range data.Profiles {
		if err := profile.Validate(); err != nil {
			return connectionsFile{}, fmt.Errorf("invalid saved connection: %w", err)
		}
	}
	return data, nil
}

func (s *ProfileStore) write(data connectionsFile) error {
	data.Version = connectionsFileVersion
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode connections: %w", err)
	}
	output = append(output, '\n')
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".connections-*.json")
	if err != nil {
		return fmt.Errorf("create temporary connections file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("protect temporary connections file: %w", err)
	}
	if _, err := temp.Write(output); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary connections file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary connections file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary connections file: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("replace connections file: %w", err)
	}
	return nil
}

func resolveProfile(data connectionsFile, reference string) (ConnectionProfile, error) {
	reference = strings.TrimSpace(reference)
	for _, profile := range data.Profiles {
		if profile.ID == reference || strings.EqualFold(profile.Name, reference) {
			return profile, nil
		}
	}
	return ConnectionProfile{}, fmt.Errorf("%w: %q", ErrConnectionNotFound, reference)
}

func mostRecentlyUsed(profiles []ConnectionProfile) ConnectionProfile {
	selected := profiles[0]
	for _, profile := range profiles[1:] {
		if profile.LastUsedAt.After(selected.LastUsedAt) {
			selected = profile
		}
	}
	return selected
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate connection id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
