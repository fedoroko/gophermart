package users

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
	"testing"
	"time"
)

func TestFromJSON(t *testing.T) {
	type args struct {
		body io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    *TempUser
		wantErr bool
	}{
		{
			name: "positive",
			args: args{
				body: strings.NewReader(`
					{"login":"gopher","password":"qwerty"}
				`),
			},
			want: &TempUser{
				Login:    "gopher",
				Password: "qwerty",
			},
			wantErr: false,
		},
		{
			name: "short login",
			args: args{
				body: strings.NewReader(`
					{"login":"","password":"qwerty"}
				`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "short password",
			args: args{
				body: strings.NewReader(`
					{"login":"gopher","password":""}
				`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "bad format #1",
			args: args{
				body: strings.NewReader(`
					{"login":"gopher"}
				`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "bad format #2",
			args: args{
				body: strings.NewReader(`
					{}
				`),
			},
			want:    nil,
			wantErr: true,
		},
		//{
		//	name: "injections",
		//	args: args{
		//		body: strings.NewReader(`
		//			{"login":"gopher\"'", "password":"qwerty';"}
		//		`),
		//	},
		//	want:    nil,
		//	wantErr: false,
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromJSON(tt.args.body)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTempSession_Commit(t *testing.T) {
	expire := time.Now().Add(time.Minute)
	type fields struct {
		Token  string
		User   *User
		Expire time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   *Session
	}{
		{
			name: "positive",
			fields: fields{
				Token: "token",
				User: &User{
					login: "gopher",
				},
				Expire: expire,
			},
			want: &Session{
				token: "token",
				user: &User{
					login: "gopher",
				},
				expire: expire,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := TempSession{
				Token:  tt.fields.Token,
				User:   tt.fields.User,
				Expire: tt.fields.Expire,
			}
			got := temp.Commit()
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTempUser_Commit(t *testing.T) {
	type fields struct {
		ID        *int64
		Login     string
		Password  string
		Balance   *float64
		Withdrawn *float64
		LastLogin *time.Time
	}
	var blankID int64 = 1
	tests := []struct {
		name   string
		fields fields
		want   *User
	}{
		{
			name: "positive",
			fields: fields{
				ID:        &blankID,
				Login:     "gopher",
				Password:  "qwerty",
				Balance:   nil,
				Withdrawn: nil,
				LastLogin: nil,
			},
			want: &User{
				id:    1,
				login: "gopher",
			},
		},
		{
			name: "blank id",
			fields: fields{
				ID:        nil,
				Login:     "gopher",
				Password:  "qwerty",
				Balance:   nil,
				Withdrawn: nil,
				LastLogin: nil,
			},
			want: &User{
				login: "gopher",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := TempUser{
				ID:        tt.fields.ID,
				Login:     tt.fields.Login,
				Password:  tt.fields.Password,
				Balance:   tt.fields.Balance,
				Withdrawn: tt.fields.Withdrawn,
				LastLogin: tt.fields.LastLogin,
			}
			got := temp.Commit()
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTempUser_ConfirmPassword(t *testing.T) {
	type fields struct {
		ID        *int
		Login     string
		Password  string
		Balance   *float64
		Withdrawn *float64
		LastLogin *time.Time
	}
	type args struct {
		t2 *TempUser
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "positive",
			fields: fields{
				Password: "qwerty",
			},
			args: args{
				t2: &TempUser{
					Password: "qwerty",
				},
			},
			want: true,
		},
		{
			name: "negative #1",
			fields: fields{
				Password: "qwerty",
			},
			args: args{
				t2: &TempUser{
					Password: "qwerty1",
				},
			},
			want: false,
		},
		{
			name: "negative #2",
			fields: fields{
				Password: "qwerty",
			},
			args: args{
				t2: &TempUser{
					Password: "111222",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := &TempUser{
				Password: tt.fields.Password,
			}

			hashed, _ := temp.HashPassword()
			temp.Password = string(*hashed)
			got := temp.ConfirmPassword(tt.args.t2)

			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTempUser_GenerateToken(t *testing.T) {
	count := 10
	stack := make(map[string]struct{}, count)
	u := TempUser{}
	for i := 1; i <= count; i++ {
		t.Run(fmt.Sprintf("no repeat #%d", i), func(t *testing.T) {
			token := u.GenerateToken()
			assert.NotContains(t, stack, token)
			stack[token] = struct{}{}
		})
	}
}

func TestTempUser_HashPassword(t *testing.T) {
	type fields struct {
		Password string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "positive #1",
			fields: fields{
				Password: "qwerty",
			},
			wantErr: false,
		},
		{
			name: "positive #2",
			fields: fields{
				Password: "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := &TempUser{
				Password: tt.fields.Password,
			}
			_, err := temp.HashPassword()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_validate(t *testing.T) {
	type args struct {
		l string
		p string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "positive",
			args: args{
				l: "gopher",
				p: "qwerty",
			},
			wantErr: false,
		},
		{
			name: "empty value",
			args: args{
				l: "",
				p: "",
			},
			wantErr: true,
		},
		{
			name: "spaces",
			args: args{
				l: "   gop   ",
				p: " qwerty ",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.args.l, tt.args.p)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
