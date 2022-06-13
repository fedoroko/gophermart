package handlers

import (
	"bytes"
	"context"
	"github.com/fedoroko/gophermart/internal/middlewares"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/rs/zerolog"
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

func testRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader, ct string, token *string) (int, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	req.Header.Set("Content-type", ct)
	if token != nil {
		req.Header.Set("Authorization", *token)
	}
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()

	return resp.StatusCode, string(respBody)
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
	var blankID int64 = 1
	user := users.TempUser{
		ID:       &blankID,
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
				err:     users.WrongPairError,
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
	l := config.NewLogger(&zerolog.Logger{})
	h := Handler(db, nil, l, time.Second*30)
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

			resp, body := testRequest(
				t, ts, http.MethodPost, "/api/user/login",
				bytes.NewReader(tt.fields.body), "application/json", nil)

			assert.Equal(t, tt.want.code, resp)
			if len(tt.want.body) > 0 {
				assert.Equal(t, tt.want.body, body)
			}
		})
	}
}

func Test_handler_LogoutFunc(t *testing.T) {
	type fields struct {
		db      storage.Repo
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
				timeout: tt.fields.timeout,
			}
			h.LogoutFunc(tt.args.c)
		})
	}
}

func Test_handler_OrderFunc(t *testing.T) {
	var blankID int64 = 1
	user := users.TempUser{
		ID:    &blankID,
		Login: "gopaher",
	}.Commit()
	type fields struct {
		body    []byte
		waitErr bool
	}
	type dbExpect struct {
		ctx   context.Context
		order *orders.Order
		err   error
	}
	type want struct {
		code int
	}
	tests := []struct {
		name     string
		fields   fields
		dbExpect dbExpect
		want     want
	}{
		{
			name: "positive",
			fields: fields{
				body:    []byte(`5512703182881200`),
				waitErr: false,
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				order: &orders.Order{
					Number: 5512703182881200,
					User:   user,
				},
				err: nil,
			},
			want: want{
				code: http.StatusAccepted,
			},
		},
		{
			name: "bad format #1",
			fields: fields{
				body: []byte(`
					{"login":"go","password":"qwerty"}
				`),
				waitErr: true,
			},
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name: "bad format #2",
			fields: fields{
				body: []byte(`
					
				`),
				waitErr: true,
			},
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name: "invalid number",
			fields: fields{
				body:    []byte(`1`),
				waitErr: true,
			},
			want: want{
				code: http.StatusUnprocessableEntity,
			},
		},
		{
			name: "already exists",
			fields: fields{
				body:    []byte(`5512703182881200`),
				waitErr: false,
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				order: &orders.Order{
					Number: 5512703182881200,
					User:   user,
				},
				err: orders.ThrowAlreadyExistsErr(),
			},
			want: want{
				code: http.StatusOK,
			},
		},
		{
			name: "belongs to other",
			fields: fields{
				body:    []byte(`5512703182881200`),
				waitErr: false,
			},
			dbExpect: dbExpect{
				ctx: context.Background(),
				order: &orders.Order{
					Number: 5512703182881200,
					User:   user,
				},
				err: orders.ThrowBelongToAnotherErr(),
			},
			want: want{
				code: http.StatusConflict,
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockRepo(ctrl)
	l := config.NewLogger(&zerolog.Logger{})
	q := mocks.NewMockQueue(ctrl)
	h := Handler(db, q, l, time.Second*30)
	r := SetUpRouter()
	r.Use(middlewares.AuthBasic(db, l))
	r.POST("/api/user/order", h.OrderFunc)

	ts := httptest.NewServer(r)
	defer ts.Close()

	token := "123"
	s := users.TempSession{
		Token:  token,
		User:   user,
		Expire: time.Now().Add(time.Minute),
	}.Commit()

	q.EXPECT().Push(gomock.Any()).Return(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.EXPECT().SessionCheck(gomock.Any(), token).Return(s, nil)
			if tt.fields.waitErr == false {
				db.EXPECT().
					OrderCreate(gomock.Any(), gomock.Any()).
					Return(tt.dbExpect.err)
			}

			resp, _ := testRequest(
				t, ts, http.MethodPost, "/api/user/order",
				bytes.NewReader(tt.fields.body), "text/plain", &token)
			assert.Equal(t, tt.want.code, resp)
		})
	}
}

//func Test_handler_OrdersFunc(t *testing.T) {
//	type fields struct {
//		r       storage.Repo
//		logger  *config.Logger
//		timeout time.Duration
//	}
//	type args struct {
//		c *gin.Context
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	db := mocks.NewMockRepo(ctrl)
//	h := Handler(db, nil, nil, time.Second*30)
//	r := SetUpRouter()
//	r.POST("/api/user/login", h.LoginFunc)
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			h.OrdersFunc(tt.args.c)
//		})
//	}
//}

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
				err:     users.ThrowAlreadyExistsErr(),
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
	l := config.NewLogger(&zerolog.Logger{})
	h := Handler(db, nil, l, time.Second*30)
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

			resp, body := testRequest(
				t, ts, http.MethodPost, "/api/user/register",
				bytes.NewReader(tt.fields.body), "application/json", nil)
			assert.Equal(t, tt.want.code, resp)
			if len(tt.want.body) > 0 {
				assert.Equal(t, tt.want.body, body)
			}
		})
	}
}

//func Test_handler_WithdrawFunc(t *testing.T) {
//	type fields struct {
//		r       storage.Repo
//		logger  *config.Logger
//		timeout time.Duration
//	}
//	type args struct {
//		c *gin.Context
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	db := mocks.NewMockRepo(ctrl)
//	h := Handler(db, nil, nil, time.Second*30)
//	r := SetUpRouter()
//	r.POST("/api/user/login", h.LoginFunc)
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			h.WithdrawFunc(tt.args.c)
//		})
//	}
//}
//
//func Test_handler_WithdrawalsFunc(t *testing.T) {
//	type fields struct {
//		r       storage.Repo
//		logger  *config.Logger
//		timeout time.Duration
//	}
//	type args struct {
//		c *gin.Context
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	db := mocks.NewMockRepo(ctrl)
//	h := Handler(db, nil, nil, time.Second*30)
//	r := SetUpRouter()
//	r.POST("/api/user/login", h.LoginFunc)
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			h := &handler{
//				logger:  tt.fields.logger,
//				timeout: tt.fields.timeout,
//			}
//			h.WithdrawalsFunc(tt.args.c)
//		})
//	}
//}
