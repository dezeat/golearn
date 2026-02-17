package localconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config persists local CLI/TUI settings.
type Config struct {
	CurrentUserID int64 `json:"current_user_id"`
}

// Store reads and writes golearn config from disk.
type Store struct {
	path string
}

// DefaultPath returns ~/.golearn/config.json.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".golearn", "config.json")
}

// NewStore creates a config store at a specific path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the configured store path.
func (s *Store) Path() string {
	return s.path
}

// Load returns config contents. Missing file returns zero-value config and no error.
func (s *Store) Load() (Config, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", s.path, err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %s: %w", s.path, err)
	}
	return cfg, nil
}

// Save writes config atomically.
func (s *Store) Save(cfg Config) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
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
