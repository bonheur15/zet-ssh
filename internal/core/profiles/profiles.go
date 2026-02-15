package profiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type AuthType string

const (
	AuthAgent    AuthType = "agent"
	AuthKey      AuthType = "key"
	AuthPassword AuthType = "password"
)

type Profile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	User        string   `json:"user"`
	AuthType    AuthType `json:"auth_type"`
	KeyPath     string   `json:"key_path,omitempty"`
	VaultKeyID  string   `json:"vault_key_id,omitempty"`
	VaultPassID string   `json:"vault_pass_id,omitempty"`
	Tags        []string `json:"tags"`
}

type Store struct {
	profiles []Profile
	path     string
	mu       sync.RWMutex
}

func NewStore(configDir string) (*Store, error) {
	path := filepath.Join(configDir, "profiles.json")
	s := &Store{
		path: path,
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			return nil, err
		}
		s.profiles = []Profile{}
		return s, s.Save()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &s.profiles)
	return s, err
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.profiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) Add(p Profile) error {
	s.mu.Lock()
	s.profiles = append(s.profiles, p)
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) List() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Profile, len(s.profiles))
	copy(out, s.profiles)
	return out
}

func (s *Store) Update(p Profile) error {
	s.mu.Lock()
	found := false
	for i := range s.profiles {
		if s.profiles[i].ID == p.ID {
			s.profiles[i] = p
			found = true
			break
		}
	}
	s.mu.Unlock()

	if !found {
		return fmt.Errorf("profile not found: %s", p.ID)
	}
	return s.Save()
}

func (s *Store) Upsert(p Profile) error {
	if p.ID == "" {
		return s.Add(p)
	}

	s.mu.Lock()
	for i := range s.profiles {
		if s.profiles[i].ID == p.ID {
			s.profiles[i] = p
			s.mu.Unlock()
			return s.Save()
		}
	}
	s.profiles = append(s.profiles, p)
	s.mu.Unlock()
	return s.Save()
}
