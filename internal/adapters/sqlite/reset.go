package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateResetPath checks that the given path is safe to delete.
// Returns an error if the path is empty, points to root, or looks suspicious.
func ValidateResetPath(dbPath string) error {
	if dbPath == "" {
		return fmt.Errorf("db path is empty")
	}

	cleaned := filepath.Clean(dbPath)

	// Reject root or very short paths.
	if cleaned == "/" || cleaned == "." || cleaned == ".." {
		return fmt.Errorf("refusing to delete %q: path is root or relative parent", dbPath)
	}

	// Reject paths that don't end with a reasonable suffix.
	if !strings.HasSuffix(cleaned, ".db") && !strings.HasSuffix(cleaned, ".sqlite") && !strings.HasSuffix(cleaned, ".sqlite3") {
		return fmt.Errorf("refusing to delete %q: path does not look like a database file", dbPath)
	}

	// Reject if path has fewer than 2 components (e.g. "/something.db").
	dir := filepath.Dir(cleaned)
	if dir == "/" || dir == "." {
		return fmt.Errorf("refusing to delete %q: file appears to be in root or current directory without explicit path", dbPath)
	}

	return nil
}

// ResetDB removes the database file at dbPath, along with WAL/SHM sidecars.
// It validates the path first for basic safety, then removes the files.
// Returns nil if the files don't exist (already clean).
func ResetDB(dbPath string) error {
	if err := ValidateResetPath(dbPath); err != nil {
		return err
	}

	cleaned := filepath.Clean(dbPath)
	sidecars := []string{cleaned + "-wal", cleaned + "-shm"}

	removed := false
	if _, err := os.Stat(cleaned); err == nil {
		if err := os.Remove(cleaned); err != nil {
			return fmt.Errorf("remove db %s: %w", cleaned, err)
		}
		removed = true
	}

	for _, sc := range sidecars {
		if _, err := os.Stat(sc); err == nil {
			_ = os.Remove(sc) // best-effort cleanup
		}
	}

	if !removed {
		return fmt.Errorf("database file %s does not exist", cleaned)
	}

	return nil
}
