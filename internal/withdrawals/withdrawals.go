package withdrawals

import (
	"encoding/json"
	"github.com/fedoroko/gophermart/internal/validation"
	"io"
	"io/ioutil"
	"strconv"
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

var NotEnoughBalanceError *notEnoughBalanceErr

type notEnoughBalanceErr struct{}

func (e *notEnoughBalanceErr) Error() string {
	return "not enough balance"
}

func ThrowNotEnoughBalanceErr() *notEnoughBalanceErr {
	return &notEnoughBalanceErr{}
}

var NoRecordsError *noRecordsErr

type noRecordsErr struct{}

func (e *noRecordsErr) Error() string {
	return "not enough balance"
}

func ThrowNoRecordsErr() *noRecordsErr {
	return &noRecordsErr{}
}

type Withdrawal struct {
	Order      int64       `json:"order"`
	User       *users.User `json:"-"`
	Sum        float64     `json:"sum"`
	UploadedAt time.Time   `json:"uploaded_at"`
}

func (w *Withdrawal) UnmarshalJSON(data []byte) error {
	raw := struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}{}

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return ThrowInvalidNumberErr()
	}

	order, err := strconv.ParseInt(raw.Order, 10, 64)
	if err != nil {
		return ThrowInvalidNumberErr()
	}

	w.Order = order
	w.Sum = raw.Sum

	return nil
}

func FromJSON(user *users.User, body io.Reader) (*Withdrawal, error) {
	w := &Withdrawal{
		User:       user,
		UploadedAt: time.Now(),
	}

	b, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &w)

	if err != nil {
		return nil, ThrowInvalidNumberErr()
	}

	if !validation.IsValid(w.Order) {
		return nil, ThrowInvalidNumberErr()
	}

	if user.Balance == nil || *user.Balance < w.Sum {
		return nil, ThrowNotEnoughBalanceErr()
	}

	return w, nil
}
