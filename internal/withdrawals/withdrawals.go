package withdrawals

import (
	"encoding/json"
	"github.com/fedoroko/gophermart/internal/validation"
	"io"
	"io/ioutil"
	"time"

	"github.com/fedoroko/gophermart/internal/users"
)

var InvalidOrderError *invalidOrderErr

type invalidOrderErr struct{}

func (e *invalidOrderErr) Error() string {
	return "invalid number"
}

func ThrowInvalidNumberErr() *invalidOrderErr {
	return &invalidOrderErr{}
}

type Withdrawal struct {
	Order      int64 `json:"order"`
	User       *users.User
	Sum        float64   `json:"sum"`
	UploadedAt time.Time `json:"uploaded_at"`
}

func FromJSON(user *users.User, body io.Reader) (*Withdrawal, error) {
	t := &Withdrawal{
		User:       user,
		UploadedAt: time.Now(),
	}

	b, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &t)
	if err != nil {
		return nil, err
	}

	if validation.IsValid(t.Order) == false {
		return nil, ThrowInvalidNumberErr()
	}

	return t, nil
}
