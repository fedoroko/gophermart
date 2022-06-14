package users

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var WrongPairError *wrongPairErr

type wrongPairErr struct{}

func (e *wrongPairErr) Error() string {
	return "wrong pair"
}

func ThrowWrongPairErr() *wrongPairErr {
	return &wrongPairErr{}
}

//

var AlreadyExistsError *alreadyExistsErr

type alreadyExistsErr struct{}

func (e *alreadyExistsErr) Error() string {
	return "login already exists"
}

func ThrowAlreadyExistsErr() *alreadyExistsErr {
	return &alreadyExistsErr{}
}

//

var BadFormatError *badFormatErr

type badFormatErr struct {
	field  string
	reason string
}

func (e *badFormatErr) Error() string {
	return fmt.Sprintf("bad format: field %s - %s", e.field, e.reason)
}

func ThrowBadFormatErr(f, r string) *badFormatErr {
	return &badFormatErr{
		field:  f,
		reason: r,
	}
}

// TempUser DB input with public fields
type TempUser struct {
	ID        *int64
	Login     string `json:"login"`
	Password  string `json:"password"`
	Balance   *float64
	Withdrawn *float64
	LastLogin *time.Time
}

func (t *TempUser) HashPassword() (*[]byte, error) {
	b := []byte(t.Password)
	hashed, err := bcrypt.GenerateFromPassword(b, bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &hashed, nil
}

func (t *TempUser) ConfirmPassword(t2 *TempUser) bool {
	hashed := []byte(t.Password)
	plain := []byte(t2.Password)
	if err := bcrypt.CompareHashAndPassword(hashed, plain); err != nil {
		return false
	}
	return true
}

func (t *TempUser) GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}

	return hex.EncodeToString(b)
}

func (t TempUser) Commit() *User {
	var id int64 = 0
	if t.ID != nil {
		id = *t.ID
	}
	return &User{
		id:        id,
		login:     t.Login,
		Balance:   t.Balance,
		Withdrawn: t.Withdrawn,
		lastLogin: t.LastLogin,
	}
}

func FromJSON(body io.Reader) (*TempUser, error) {
	var t TempUser
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &t)
	if err != nil {
		return nil, err
	}

	if err = validate(t.Login, t.Password); err != nil {
		return nil, err
	}

	return &t, nil
}

func validate(l, p string) error {
	if len(strings.TrimSpace(l)) < 6 {
		return ThrowBadFormatErr("login", "should be at least 6 symbols len")
	}

	if len(strings.TrimSpace(p)) < 6 {
		return ThrowBadFormatErr("password", "should be at least 6 symbols len")
	}

	return nil
}

// TempSession DB input with public fields
type TempSession struct {
	Token  string
	User   *User
	Expire time.Time
}

func (t TempSession) Commit() *Session {
	return &Session{
		token:  t.Token,
		user:   t.User,
		expire: t.Expire,
	}
}
