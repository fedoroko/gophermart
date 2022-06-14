package users

import (
	"time"
)

// User DB output
type User struct {
	id        int64
	login     string
	Balance   *float64 `json:"current,omitempty"`
	Withdrawn *float64 `json:"withdrawn,omitempty"`
	lastLogin *time.Time
}

func (u *User) ID() int64 {
	return u.id
}

func (u *User) Login() string {
	return u.login
}

func (u *User) LastLogin() *time.Time {
	return u.lastLogin
}
