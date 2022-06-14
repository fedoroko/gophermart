package controllers

import (
	"context"
	"github.com/fedoroko/gophermart/internal/accrual"
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/mocks"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/fedoroko/gophermart/internal/storage"
	"github.com/fedoroko/gophermart/internal/users"
	"github.com/fedoroko/gophermart/internal/withdrawals"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"io"
	"reflect"
	"testing"
)

func getCtrl(t *testing.T) (Controller, *gomock.Controller, *mocks.MockRepo) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockRepo(ctrl)
	l := config.NewLogger(&zerolog.Logger{})
	q := mocks.NewMockQueue(ctrl)
	return &controller{
		r:      db,
		logger: l,
		q:      q,
	}, ctrl, db
}

//func Test_controller_Login(t *testing.T) {
//	type args struct {
//		body   io.Reader
//		retErr error
//	}
//	tests := []struct {
//		name string
//		args args
//		want error
//	}{
//		{
//			name: "positive",
//			args: args{
//				body:   strings.NewReader(`{"login":"gopher", "password":"qwerty"}`),
//				retErr: nil,
//			},
//			want: nil,
//		},
//		{
//			name: "bad format",
//			args: args{
//				body:   strings.NewReader(`{"password":"qwerty"}`),
//				retErr: nil,
//			},
//			want: users.ThrowBadFormatErr("", ""),
//		},
//		{
//			name: "wrong pair",
//			args: args{
//				strings.NewReader(`{"login":"gopher", "password":"qwerty1"}`),
//			},
//			want: users.ThrowWrongPairErr(),
//		},
//	}
//
//	c, ctrl, db := getCtrl(t)
//	defer ctrl.Finish()
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			db.EXPECT().UserCreate(gomock.Any(), gomock.Any()).Return("", nil)
//			_, err := c.Login(context.Background(), tt.args.body)
//			assert.ErrorAs(t, err, tt.want)
//		})
//	}
//}

func Test_controller_Logout(t *testing.T) {
	type fields struct {
		r      storage.Repo
		q      accrual.Queue
		logger *config.Logger
	}
	type args struct {
		ctx context.Context
		s   *users.Session
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{
				r:      tt.fields.r,
				q:      tt.fields.q,
				logger: tt.fields.logger,
			}
			if err := c.Logout(tt.args.ctx, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Logout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_controller_Order(t *testing.T) {
	type fields struct {
		r      storage.Repo
		q      accrual.Queue
		logger *config.Logger
	}
	type args struct {
		ctx  context.Context
		u    *users.User
		body io.Reader
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{
				r:      tt.fields.r,
				q:      tt.fields.q,
				logger: tt.fields.logger,
			}
			if err := c.Order(tt.args.ctx, tt.args.u, tt.args.body); (err != nil) != tt.wantErr {
				t.Errorf("Order() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_controller_Orders(t *testing.T) {
	type fields struct {
		r      storage.Repo
		q      accrual.Queue
		logger *config.Logger
	}
	type args struct {
		ctx context.Context
		u   *users.User
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*orders.Order
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{
				r:      tt.fields.r,
				q:      tt.fields.q,
				logger: tt.fields.logger,
			}
			got, err := c.Orders(tt.args.ctx, tt.args.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("Orders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Orders() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controller_Register(t *testing.T) {
	type fields struct {
		r      storage.Repo
		q      accrual.Queue
		logger *config.Logger
	}
	type args struct {
		ctx  context.Context
		body io.Reader
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{
				r:      tt.fields.r,
				q:      tt.fields.q,
				logger: tt.fields.logger,
			}
			got, err := c.Register(tt.args.ctx, tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Register() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controller_Withdraw(t *testing.T) {
	type fields struct {
		r      storage.Repo
		q      accrual.Queue
		logger *config.Logger
	}
	type args struct {
		ctx  context.Context
		u    *users.User
		body io.Reader
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{
				r:      tt.fields.r,
				q:      tt.fields.q,
				logger: tt.fields.logger,
			}
			if err := c.Withdraw(tt.args.ctx, tt.args.u, tt.args.body); (err != nil) != tt.wantErr {
				t.Errorf("Withdraw() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_controller_Withdrawals(t *testing.T) {
	type fields struct {
		r      storage.Repo
		q      accrual.Queue
		logger *config.Logger
	}
	type args struct {
		ctx context.Context
		u   *users.User
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*withdrawals.Withdrawal
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{
				r:      tt.fields.r,
				q:      tt.fields.q,
				logger: tt.fields.logger,
			}
			got, err := c.Withdrawals(tt.args.ctx, tt.args.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("Withdrawals() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Withdrawals() got = %v, want %v", got, tt.want)
			}
		})
	}
}
