package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/mocks"
	"github.com/fedoroko/gophermart/internal/storage"
	"github.com/fedoroko/gophermart/internal/users"
)

func SetUpRouter() *gin.Engine {
	router := gin.Default()
	return router
}

func testRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()

	return resp, string(respBody)
}

func Test_handler_LoginFunc(t *testing.T) {
	type fields struct {
		body []byte
	}
	type dbExpect struct {
		ctx     context.Context
		user    *users.TempUser
		session *users.Session
		err     error
	}
	type want struct {
		code int
		body string
	}
	user := users.TempUser{
		Login:    "gopher",
		Password: "qwerty",
	}.Commit()
	session := users.TempSession{
		Token:  "token123",
		User:   user,
		Expire: time.Now().Add(time.Minute * 3),
	}.Commit()
	tests := []struct {
		name     string
		fields   fields
		dbExpect dbExpect
		want     want
	}{
		{
			name: "positive",
			fields: fields{
				body: []byte(`
					{"login":"gopher","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				user: &users.TempUser{
					Login:    "gopher",
					Password: "qwerty",
				},
				session: session,
				err:     nil,
			},
			want: want{
				code: http.StatusOK,
				body: "\"token123\"",
			},
		},
		{
			name: "bad format #1",
			fields: fields{
				body: []byte(`
					{"login":"go","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx:     context.Background(),
				user:    nil,
				session: nil,
				err:     nil,
			},
			want: want{
				code: http.StatusBadRequest,
				body: "",
			},
		},
		{
			name: "bad format #2",
			fields: fields{
				body: []byte(`
					{}
				`),
			},
			dbExpect: dbExpect{
				ctx:     context.Background(),
				user:    nil,
				session: nil,
				err:     nil,
			},
			want: want{
				code: http.StatusBadRequest,
				body: "",
			},
		},
		{
			name: "wrong pair",
			fields: fields{
				body: []byte(`
					{"login":"gopher","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				user: &users.TempUser{
					Login:    "gopher",
					Password: "qwerty",
				},
				session: nil,
				err:     errors.New(" no rows "),
			},
			want: want{
				code: http.StatusUnauthorized,
				body: "",
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockRepo(ctrl)
	h := Handler(db, nil, time.Second*30)
	r := SetUpRouter()
	r.POST("/api/user/login", h.LoginFunc)

	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dbExpect.user != nil {
				db.EXPECT().
					UserExists(gomock.Any(), tt.dbExpect.user).
					Return(tt.dbExpect.session, tt.dbExpect.err)
			}

			resp, body := testRequest(t, ts, http.MethodPost, "/api/user/login", bytes.NewReader(tt.fields.body))
			assert.Equal(t, tt.want.code, resp.StatusCode)
			if len(tt.want.body) > 0 {
				assert.Equal(t, tt.want.body, body)
			}
		})
	}
}

func Test_handler_LogoutFunc(t *testing.T) {
	type fields struct {
		r       storage.Repo
		logger  *config.Logger
		timeout time.Duration
	}
	type args struct {
		c *gin.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				r:       tt.fields.r,
				logger:  tt.fields.logger,
				timeout: tt.fields.timeout,
			}
			h.LogoutFunc(tt.args.c)
		})
	}
}

func Test_handler_OrderFunc(t *testing.T) {
	type fields struct {
		body []byte
	}
	type dbExpect struct {
		ctx     context.Context
		user    *users.TempUser
		session *users.Session
		err     error
	}
	type want struct {
		code int
		body string
	}
	user := users.TempUser{
		Login:    "gopher",
		Password: "qwerty",
	}.Commit()
	session := users.TempSession{
		Token:  "token123",
		User:   user,
		Expire: time.Now().Add(time.Minute * 3),
	}.Commit()
	tests := []struct {
		name     string
		fields   fields
		dbExpect dbExpect
		want     want
	}{
		{
			name: "positive",
			fields: fields{
				body: []byte(`
					{"login":"gopher","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				user: &users.TempUser{
					Login:    "gopher",
					Password: "qwerty",
				},
				session: session,
				err:     nil,
			},
			want: want{
				code: http.StatusOK,
				body: "\"token123\"",
			},
		},
		{
			name: "bad format #1",
			fields: fields{
				body: []byte(`
					{"login":"go","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx:     context.Background(),
				user:    nil,
				session: nil,
				err:     nil,
			},
			want: want{
				code: http.StatusBadRequest,
				body: "",
			},
		},
		{
			name: "bad format #2",
			fields: fields{
				body: []byte(`
					{}
				`),
			},
			dbExpect: dbExpect{
				ctx:     context.Background(),
				user:    nil,
				session: nil,
				err:     nil,
			},
			want: want{
				code: http.StatusBadRequest,
				body: "",
			},
		},
		{
			name: "wrong pair",
			fields: fields{
				body: []byte(`
					{"login":"gopher","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				user: &users.TempUser{
					Login:    "gopher",
					Password: "qwerty",
				},
				session: nil,
				err:     errors.New(" no rows "),
			},
			want: want{
				code: http.StatusUnauthorized,
				body: "",
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockRepo(ctrl)
	h := Handler(db, nil, time.Second*30)
	r := SetUpRouter()
	r.POST("/api/user/login", h.LoginFunc)

	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dbExpect.user != nil {
				db.EXPECT().
					UserExists(gomock.Any(), tt.dbExpect.user).
					Return(tt.dbExpect.session, tt.dbExpect.err)
			}

			resp, body := testRequest(t, ts, http.MethodPost, "/api/user/login", bytes.NewReader(tt.fields.body))
			assert.Equal(t, tt.want.code, resp.StatusCode)
			if len(tt.want.body) > 0 {
				assert.Equal(t, tt.want.body, body)
			}
		})
	}
}

func Test_handler_OrdersFunc(t *testing.T) {
	type fields struct {
		r       storage.Repo
		logger  *config.Logger
		timeout time.Duration
	}
	type args struct {
		c *gin.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				r:       tt.fields.r,
				logger:  tt.fields.logger,
				timeout: tt.fields.timeout,
			}
			h.OrdersFunc(tt.args.c)
		})
	}
}

func Test_handler_RegisterFunc(t *testing.T) {
	type fields struct {
		body []byte
	}
	type dbExpect struct {
		ctx     context.Context
		user    *users.TempUser
		session *users.Session
		err     error
	}
	type want struct {
		code int
		body string
	}
	user := users.TempUser{
		Login:    "gopher",
		Password: "qwerty",
	}.Commit()
	session := users.TempSession{
		Token:  "token123",
		User:   user,
		Expire: time.Now().Add(time.Minute * 3),
	}.Commit()
	tests := []struct {
		name     string
		fields   fields
		dbExpect dbExpect
		want     want
	}{
		{
			name: "positive",
			fields: fields{
				body: []byte(`
					{"login":"gopher","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				user: &users.TempUser{
					Login:    "gopher",
					Password: "qwerty",
				},
				session: session,
				err:     nil,
			},
			want: want{
				code: http.StatusOK,
				body: "\"token123\"",
			},
		},
		{
			name: "bad format #1",
			fields: fields{
				body: []byte(`
					{"login":"go","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx:     context.Background(),
				user:    nil,
				session: nil,
				err:     nil,
			},
			want: want{
				code: http.StatusBadRequest,
				body: "",
			},
		},
		{
			name: "bad format #2",
			fields: fields{
				body: []byte(`
					{}
				`),
			},
			dbExpect: dbExpect{
				ctx:     context.Background(),
				user:    nil,
				session: nil,
				err:     nil,
			},
			want: want{
				code: http.StatusBadRequest,
				body: "",
			},
		},
		{
			name: "already exists",
			fields: fields{
				body: []byte(`
					{"login":"gopher","password":"qwerty"}
				`),
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				user: &users.TempUser{
					Login:    "gopher",
					Password: "qwerty",
				},
				session: nil,
				err:     errors.New("duplicate key"),
			},
			want: want{
				code: http.StatusConflict,
				body: "",
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockRepo(ctrl)
	h := Handler(db, nil, time.Second*30)
	r := SetUpRouter()
	r.POST("/api/user/register", h.RegisterFunc)

	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dbExpect.user != nil {
				db.EXPECT().
					UserCreate(gomock.Any(), tt.dbExpect.user).
					Return(tt.dbExpect.session, tt.dbExpect.err)
			}

			resp, body := testRequest(t, ts, http.MethodPost, "/api/user/register", bytes.NewReader(tt.fields.body))
			assert.Equal(t, tt.want.code, resp.StatusCode)
			if len(tt.want.body) > 0 {
				assert.Equal(t, tt.want.body, body)
			}
		})
	}
}

func Test_handler_WithdrawFunc(t *testing.T) {
	type fields struct {
		r       storage.Repo
		logger  *config.Logger
		timeout time.Duration
	}
	type args struct {
		c *gin.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				r:       tt.fields.r,
				logger:  tt.fields.logger,
				timeout: tt.fields.timeout,
			}
			h.WithdrawFunc(tt.args.c)
		})
	}
}

func Test_handler_WithdrawalsFunc(t *testing.T) {
	type fields struct {
		r       storage.Repo
		logger  *config.Logger
		timeout time.Duration
	}
	type args struct {
		c *gin.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				r:       tt.fields.r,
				logger:  tt.fields.logger,
				timeout: tt.fields.timeout,
			}
			h.WithdrawalsFunc(tt.args.c)
		})
	}
}
