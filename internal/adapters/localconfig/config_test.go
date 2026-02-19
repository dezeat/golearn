package localconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/localconfig"
	"github.com/dezeat/golearn/internal/ports"
)

func TestStore_LoadMissingFile(t *testing.T) {
	store := localconfig.NewStore(filepath.Join(t.TempDir(), "does-not-exist.json"))
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.CurrentUserID != 0 {
		t.Errorf("expected zero-value config, got CurrentUserID=%d", cfg.CurrentUserID)
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "subdir", "config.json")
	store := localconfig.NewStore(p)

	want := ports.LocalConfig{CurrentUserID: 42}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestStore_OverwriteExisting(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	store := localconfig.NewStore(p)

	if err := store.Save(ports.LocalConfig{CurrentUserID: 1}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := store.Save(ports.LocalConfig{CurrentUserID: 2}); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.CurrentUserID != 2 {
		t.Errorf("expected 2, got %d", cfg.CurrentUserID)
	}
}

func TestStore_LoadInvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(p, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := localconfig.NewStore(p)
	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStore_Path(t *testing.T) {
	want := "/tmp/test/config.json"
	store := localconfig.NewStore(want)
	if got := store.Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestStore_ImplementsConfigStore(t *testing.T) {
	store := localconfig.NewStore("/tmp/test.json")
	var _ ports.ConfigStore = store
}
