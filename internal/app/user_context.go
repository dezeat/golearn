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

package app

// CurrentUserProvider provides read/write access to the active local user profile.
type CurrentUserProvider interface {
	CurrentUserID() int64
	SetCurrentUserID(id int64)
}

// UserContext stores the current user ID in memory.
type UserContext struct {
	currentUserID int64
}

// NewUserContext creates a UserContext with an initial user ID.
func NewUserContext(initialUserID int64) *UserContext {
	return &UserContext{currentUserID: initialUserID}
}

// CurrentUserID returns the active user ID.
func (c *UserContext) CurrentUserID() int64 {
	return c.currentUserID
}

// SetCurrentUserID updates the active user ID.
func (c *UserContext) SetCurrentUserID(id int64) {
	c.currentUserID = id
}
