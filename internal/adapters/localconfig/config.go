// Package localconfig implements persistent local configuration storage
// for golearn, using a JSON file at ~/.golearn/config.json.
package localconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dezeat/golearn/internal/ports"
)

// DefaultPath returns ~/.golearn/config.json.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".golearn", "config.json")
}

// Store reads and writes golearn config from disk.
// Implements ports.ConfigStore.
type Store struct {
	path string
}

// NewStore creates a config store at a specific path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the configured store path.
func (s *Store) Path() string {
	return s.path
}

// configJSON is the on-disk JSON representation.
type configJSON struct {
	CurrentUserID int64 `json:"current_user_id"`
}

// Load returns config contents. Missing file returns zero-value config and no error.
func (s *Store) Load() (ports.LocalConfig, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return ports.LocalConfig{}, nil
	}
	if err != nil {
		return ports.LocalConfig{}, fmt.Errorf("read config %s: %w", s.path, err)
	}

	var raw configJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return ports.LocalConfig{}, fmt.Errorf("decode config %s: %w", s.path, err)
	}
	return ports.LocalConfig{CurrentUserID: raw.CurrentUserID}, nil
}

// Save writes config atomically.
func (s *Store) Save(cfg ports.LocalConfig) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}

	raw := configJSON{CurrentUserID: cfg.CurrentUserID}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	b = append(b, '\n')

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write temp config %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace config %s: %w", s.path, err)
	}
	return nil
}
