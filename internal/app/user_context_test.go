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

package app_test

import (
	"testing"

	"github.com/dezeat/golearn/internal/app"
)

func TestUserContext_InitialValue(t *testing.T) {
	ctx := app.NewUserContext(42)
	if got := ctx.CurrentUserID(); got != 42 {
		t.Errorf("CurrentUserID() = %d, want 42", got)
	}
}

func TestUserContext_SetCurrentUserID(t *testing.T) {
	ctx := app.NewUserContext(0)
	ctx.SetCurrentUserID(99)
	if got := ctx.CurrentUserID(); got != 99 {
		t.Errorf("after SetCurrentUserID(99), got %d", got)
	}
}

func TestUserContext_ZeroInitial(t *testing.T) {
	ctx := app.NewUserContext(0)
	if got := ctx.CurrentUserID(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestUserContext_ImplementsInterface(t *testing.T) {
	ctx := app.NewUserContext(1)
	// Compile-time check: *UserContext implements CurrentUserProvider.
	var _ app.CurrentUserProvider = ctx
}
