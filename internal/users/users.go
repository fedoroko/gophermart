package users

import (
	"time"
)

// User DB output
type User struct {
	id        int64
	login     string
	balance   *float64
	withdrawn *float64
	lastLogin *time.Time
}

func (u *User) Id() int64 {
	return u.id
}

func (u *User) Login() string {
	return u.login
}

func (u *User) Balance() *float64 {
	return u.balance
}

func (u *User) Withdrawn() *float64 {
	return u.withdrawn
}
