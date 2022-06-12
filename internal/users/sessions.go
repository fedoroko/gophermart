package users

import (
	"time"
)

type Session struct {
	user   *User
	token  string
	expire time.Time
}

func (s *Session) User() *User {
	return s.user
}

func (s *Session) Token() string {
	return s.token
}

func (s *Session) Expire() time.Time {
	return s.expire
}
