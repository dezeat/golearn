package sqlite_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/sqlite"
)

func TestValidateResetPath_Safety(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty path", "", true},
		{"root slash", "/", true},
		{"dot", ".", true},
		{"parent", "..", true},
		{"root db file", "/something.db", true},
		{"no db extension", "/home/user/data", true},
		{"valid path", "/home/user/.golearn/golearn.db", false},
		{"valid sqlite extension", "/tmp/test/data.sqlite", false},
		{"valid sqlite3 extension", "/tmp/test/data.sqlite3", false},
		{"relative without dir", "golearn.db", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sqlite.ValidateResetPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResetPath(%q) error = %v, wantErr = %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestResetDB_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create the DB file.
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	// Verify it exists.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("db file should exist before reset")
	}

	// Reset it.
	if err := sqlite.ResetDB(dbPath); err != nil {
		t.Fatalf("ResetDB: %v", err)
	}

	// Verify it's gone.
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("db file should not exist after reset")
	}
}

func TestResetDB_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nonexistent.db")

	err := sqlite.ResetDB(dbPath)
	if err == nil {
		t.Error("expected error when resetting nonexistent db")
	}
}
