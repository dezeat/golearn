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
	"time"

	"github.com/dezeat/golearn/internal/domain"
)

// UserRepo implements ports.UserRepository using SQLite.
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo creates a UserRepo.
func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a user profile.
func (r *UserRepo) Create(handle, displayName string) (*domain.User, error) {
	now := time.Now().UTC()
	res, err := r.db.Exec(
		`INSERT INTO users (handle, display_name, created_at) VALUES (?, ?, ?)`,
		handle, displayName, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("user last insert id: %w", err)
	}

	return &domain.User{ID: id, Handle: handle, DisplayName: displayName, CreatedAt: now}, nil
}

// GetByHandle returns a user by handle.
func (r *UserRepo) GetByHandle(handle string) (*domain.User, bool, error) {
	var u domain.User
	err := r.db.QueryRow(
		`SELECT id, handle, COALESCE(display_name, ''), created_at FROM users WHERE handle = ?`,
		handle,
	).Scan(&u.ID, &u.Handle, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get user by handle %q: %w", handle, err)
	}
	return &u, true, nil
}

// List returns all users ordered by handle.
func (r *UserRepo) List() ([]domain.User, error) {
	rows, err := r.db.Query(`SELECT id, handle, COALESCE(display_name, ''), created_at FROM users ORDER BY handle ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := make([]domain.User, 0)
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Handle, &u.DisplayName, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

// GetByID returns a user by ID.
func (r *UserRepo) GetByID(id int64) (*domain.User, bool, error) {
	var u domain.User
	err := r.db.QueryRow(
		`SELECT id, handle, COALESCE(display_name, ''), created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Handle, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get user by id %d: %w", id, err)
	}
	return &u, true, nil
}
