package orders

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/fedoroko/gophermart/internal/users"
	"github.com/fedoroko/gophermart/internal/validation"
)

type Order struct {
	Number     int64       `json:"-"`
	User       *users.User `json:"-"`
	Status     int         `json:"-"`
	Accrual    *float64    `json:"accrual,omitempty"`
	UploadedAt time.Time   `json:"-"`
}

func (o *Order) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Number     string   `json:"number"`
		Status     string   `json:"status"`
		Accrual    *float64 `json:"accrual,omitempty"`
		UploadedAt string   `json:"uploaded_at"`
	}{
		Number:     fmt.Sprintf("%d", o.Number),
		Status:     statusDecode(o.Status),
		Accrual:    o.Accrual,
		UploadedAt: o.UploadedAt.Format(time.RFC3339),
	})
}

func FromPlain(user *users.User, body io.Reader) (*Order, error) {
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	number, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return nil, ThrowInvalidRequestErr()
	}

	if !validation.IsValid(number) {
		return nil, ThrowInvalidNumberErr()
	}
	return &Order{
		Number:     number,
		User:       user,
		UploadedAt: time.Now(),
		Status:     1,
	}, nil
}

func statusDecode(status int) string {
	table := map[int]string{
		1: "NEW",
		2: "PROCESSING",
		3: "PROCESSED",
		4: "INVALID",
	}

	return table[status]
}

type QueueOrder struct {
	Number  int64    `json:"number"`
	Status  int      `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

func (o *Order) ToQueue() QueueOrder {
	return QueueOrder{
		Number:  o.Number,
		Status:  o.Status,
		Accrual: o.Accrual,
	}
}

func (q *QueueOrder) ToOrder() *Order {
	return &Order{
		Number:  q.Number,
		Status:  q.Status,
		Accrual: q.Accrual,
	}
}
