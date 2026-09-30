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

package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
)

// D-014: golearn never silently destroys user data. A schema this binary
// cannot migrate forward is refused, and the only destructive path is the
// explicit `golearn db reset --yes`.
var (
	// ErrIncompatibleSchema reports a database whose shape this binary cannot
	// safely migrate forward.
	ErrIncompatibleSchema = errors.New("incompatible database schema")

	// ErrNewerSchema reports a database written by a later golearn than this
	// one. Migrations are forward-only, so this binary has no way back.
	ErrNewerSchema = errors.New("database schema is newer than this binary")
)

// recoveryHint names the one consented destructive path. A refusal without it
// leaves the user holding a database they can neither open nor knowingly
// discard, which is how a safety measure turns into a dead end.
const recoveryHint = "back up the file if it matters, then run: golearn db reset --yes"

// coreTables are the tables a golearn database is expected to own. Their
// presence without migration tracking is what identifies a database written
// before versioning existed.
var coreTables = []string{"users", "topics", "questions", "sessions", "attempts"}

// requiredColumns are columns the repositories read unconditionally. A tracked
// database missing one was written by a build whose migration numbering no
// longer matches this binary's, so its recorded version cannot be trusted.
var requiredColumns = []struct{ table, column string }{
	{"sessions", "user_id"},
	{"attempts", "user_id"},
}

// guardSchemaCompatibility decides whether this binary may migrate the database
// it just opened. It runs before any migration so that a refusal leaves the
// file exactly as it was found.
func guardSchemaCompatibility(db *sql.DB) error {
	tracked, err := tableExists(db, "schema_migrations")
	if err != nil {
		return fmt.Errorf("check migration tracking: %w", err)
	}

	if !tracked {
		untrackedCore, err := anyTableExists(db, coreTables)
		if err != nil {
			return err
		}
		if untrackedCore != "" {
			return fmt.Errorf(
				"%w: found table %q but no migration history, so this database predates schema versioning and its exact shape is unknown; %s",
				ErrIncompatibleSchema, untrackedCore, recoveryHint)
		}
		// No tracking and no core tables: a fresh file. Migrations create it.
		return nil
	}

	if err := guardNotNewer(db); err != nil {
		return err
	}
	return guardRequiredColumns(db)
}

func guardNotNewer(db *sql.DB) error {
	var recorded sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&recorded); err != nil {
		return fmt.Errorf("read recorded schema version: %w", err)
	}
	if !recorded.Valid {
		return nil
	}
	if known := int64(len(migrations)); recorded.Int64 > known {
		return fmt.Errorf(
			"%w: database is at schema version %d but this golearn only knows up to %d; upgrade golearn to open it, or %s",
			ErrNewerSchema, recorded.Int64, known, recoveryHint)
	}
	return nil
}

func guardRequiredColumns(db *sql.DB) error {
	for _, req := range requiredColumns {
		exists, err := tableExists(db, req.table)
		if err != nil {
			return fmt.Errorf("check table %s: %w", req.table, err)
		}
		if !exists {
			// Absent entirely is fine: the migration that creates it has not
			// run yet, and it creates the column with the table.
			continue
		}
		hasColumn, err := tableHasColumn(db, req.table, req.column)
		if err != nil {
			return fmt.Errorf("check %s.%s: %w", req.table, req.column, err)
		}
		if !hasColumn {
			return fmt.Errorf(
				"%w: table %q is missing column %q, so its recorded schema version does not describe it; %s",
				ErrIncompatibleSchema, req.table, req.column, recoveryHint)
		}
	}
	return nil
}

// anyTableExists returns the first table from names present in the database, or
// an empty string. Naming the table found makes the refusal diagnosable.
func anyTableExists(db *sql.DB, names []string) (string, error) {
	for _, name := range names {
		exists, err := tableExists(db, name)
		if err != nil {
			return "", fmt.Errorf("check table %s: %w", name, err)
		}
		if exists {
			return name, nil
		}
	}
	return "", nil
}
