package orders

import (
	"github.com/fedoroko/gophermart/internal/users"
	"io"
	"io/ioutil"
	"strconv"
	"time"
)

type Order struct {
	number       int64
	user         *users.User
	isWithdrawal bool
	status       int
	sum          *float64
	uploadedAt   time.Time
}

func (o *Order) User() *users.User {
	return o.user
}

func (o *Order) Number() int64 {
	return o.number
}

func (o *Order) UploadedAt() time.Time {
	return o.uploadedAt
}

func (o *Order) Status() int {
	return o.status
}

func (o *Order) Sum() *float64 {
	return o.sum
}

func (o *Order) Process() error {
	return nil
}

type TempOrder struct {
	Number       int64
	User         *users.User
	IsWithdrawal bool
	Status       int
	Sum          *float64
	UploadedAt   time.Time
}

func (t *TempOrder) Commit() *Order {
	return &Order{
		number:       t.Number,
		user:         t.User,
		isWithdrawal: t.IsWithdrawal,
		status:       t.Status,
		sum:          t.Sum,
		uploadedAt:   t.UploadedAt,
	}
}

func FromPlain(user *users.User, body io.Reader, isWithdrawal bool) (*TempOrder, error) {
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	number, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return nil, ThrowInvalidRequestErr()
	}

	if err = validate(number); err != nil {
		return nil, err
	}

	return &TempOrder{
		Number:       number,
		User:         user,
		IsWithdrawal: isWithdrawal,
		UploadedAt:   time.Now(),
		Status:       1,
	}, nil
}

func validate(number int64) error {
	if false {
		return ThrowInvalidNumberErr()
	}
	return nil
}
