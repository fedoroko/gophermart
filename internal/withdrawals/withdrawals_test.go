package withdrawals

import (
	"github.com/fedoroko/gophermart/internal/users"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func floatToPointer(f float64) *float64 {
	return &f
}

func TestFromJSON(t *testing.T) {
	type args struct {
		user *users.User
		body io.Reader
	}
	tests := []struct {
		name string
		args args
		want *Withdrawal
		err  error
	}{
		{
			name: "positive",
			args: args{
				user: users.TempUser{
					Login:   "gopher",
					Balance: floatToPointer(1000),
				}.Commit(),
				body: strings.NewReader(`{"order":"2375460850", "sum":500}`),
			},
			want: &Withdrawal{
				Order: 2375460850,
				Sum:   500,
				User: users.TempUser{
					Login:   "gopher",
					Balance: floatToPointer(1000),
				}.Commit(),
			},
			err: nil,
		},
		{
			name: "not enough balance",
			args: args{
				user: users.TempUser{
					Login:   "gopher",
					Balance: floatToPointer(500),
				}.Commit(),
				body: strings.NewReader(`{"order":"2375460850", "sum":1000}`),
			},
			want: nil,
			err:  ThrowNotEnoughBalanceErr(),
		},
		{
			name: "invalid number",
			args: args{
				user: users.TempUser{
					Login:   "gopher",
					Balance: floatToPointer(500),
				}.Commit(),
				body: strings.NewReader(`{"order":"2375460851", "sum":100}`),
			},
			want: nil,
			err:  ThrowInvalidNumberErr(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromJSON(tt.args.user, tt.args.body)
			if err != nil {
				assert.ErrorAs(t, err, &tt.err)
			}
			if got != nil {
				assert.Equal(t, got.Order, tt.want.Order)
				assert.Equal(t, got.Sum, tt.want.Sum)
			}
		})
	}
}

func TestWithdrawal_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Order int64
		Sum   float64
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "positive",
			fields: fields{
				Order: 2375460850,
				Sum:   500,
			},
			args: args{
				data: []byte(`{"order":"2375460850","sum":500}`),
			},
			wantErr: false,
		},
		{
			name:   "bad json",
			fields: fields{},
			args: args{
				data: []byte(`{"order":"2375460850",sum:500`),
			},
			wantErr: true,
		},
		{
			name:   "too long",
			fields: fields{},
			args: args{
				data: []byte(`{"order":"237546085023754608502375460850","sum":500}`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Withdrawal{}

			if err := w.UnmarshalJSON(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.fields.Order, w.Order)
			assert.Equal(t, tt.fields.Sum, w.Sum)
		})
	}
}
