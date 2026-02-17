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
