package app

import (
	"fmt"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// ResolveUser finds the current user from the repository.
// It first tries the configuredUserID, then falls back to the "local"
// handle, then to the first available user.
// Returns the resolved user, whether it matched the configured ID, and any error.
func ResolveUser(userRepo ports.UserRepository, configuredUserID int64) (*domain.User, bool, error) {
	if configuredUserID > 0 {
		u, found, err := userRepo.GetByID(configuredUserID)
		if err != nil {
			return nil, false, fmt.Errorf("get configured user %d: %w", configuredUserID, err)
		}
		if found {
			return u, true, nil
		}
	}

	localUser, found, err := userRepo.GetByHandle("local")
	if err != nil {
		return nil, false, fmt.Errorf("get local user: %w", err)
	}
	if found {
		return localUser, false, nil
	}

	users, err := userRepo.List()
	if err != nil {
		return nil, false, fmt.Errorf("list users: %w", err)
	}
	if len(users) == 0 {
		return nil, false, fmt.Errorf("no users available")
	}
	return &users[0], false, nil
}

// EnsureDefaultUser lists existing users and seeds a "local" profile if
// none exist.  Returns the full user list (always ≥ 1 element).
func EnsureDefaultUser(userRepo ports.UserRepository) ([]domain.User, error) {
	users, err := userRepo.List()
	if err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}
	if len(users) == 0 {
		u, err := userRepo.Create("local", "Local")
		if err != nil {
			return nil, fmt.Errorf("seed local user: %w", err)
		}
		users = append(users, *u)
	}
	return users, nil
}
