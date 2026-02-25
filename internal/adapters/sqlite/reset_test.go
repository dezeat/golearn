// Copyright 2026 dezeat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

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
