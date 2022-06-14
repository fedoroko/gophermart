package orders

import (
	"fmt"
	"github.com/fedoroko/gophermart/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
	"testing"
	"time"
)

func TestFromPlain(t *testing.T) {
	type args struct {
		user *users.User
		body io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    *Order
		wantErr bool
	}{
		{
			name: "positive",
			args: args{
				user: nil,
				body: strings.NewReader(`2375460850`),
			},
			want: &Order{
				Number: 2375460850,
			},
			wantErr: false,
		},
		{
			name: "invalid",
			args: args{
				user: nil,
				body: strings.NewReader(`2375460851`),
			},
			want:    &Order{},
			wantErr: true,
		},
		{
			name: "too long",
			args: args{
				user: nil,
				body: strings.NewReader(`237546085123754608512375460851`),
			},
			want:    &Order{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromPlain(tt.args.user, tt.args.body)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, got.Number, tt.want.Number)
			}
		})
	}
}

func TestOrder_MarshalJSON(t *testing.T) {
	type fields struct {
		Number     int64
		Status     int
		UploadedAt time.Time
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name: "positive #1",
			fields: fields{
				Number:     2375460851,
				Status:     2,
				UploadedAt: time.Now(),
			},
			want: []byte(
				fmt.Sprintf(`{"number":2375460851,"status":"PROCESSING","uploaded_at":"%s"}`,
					time.Now().Format(time.RFC3339))),
			wantErr: false,
		},
		{
			name: "positive #2",
			fields: fields{
				Number:     2375460851,
				Status:     4,
				UploadedAt: time.Now(),
			},
			want: []byte(
				fmt.Sprintf(`{"number":2375460851,"status":"INVALID","uploaded_at":"%s"}`,
					time.Now().Format(time.RFC3339))),
			wantErr: false,
		},
		{
			name: "positive #3",
			fields: fields{
				Number:     2375460851,
				Status:     1,
				UploadedAt: time.Now(),
			},
			want: []byte(
				fmt.Sprintf(`{"number":2375460851,"status":"NEW","uploaded_at":"%s"}`,
					time.Now().Format(time.RFC3339))),
			wantErr: false,
		},
		{
			name: "positive #1",
			fields: fields{
				Number:     2375460851,
				Status:     3,
				UploadedAt: time.Now(),
			},
			want: []byte(
				fmt.Sprintf(`{"number":2375460851,"status":"PROCESSED","uploaded_at":"%s"}`,
					time.Now().Format(time.RFC3339))),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Order{
				Number:     tt.fields.Number,
				Status:     tt.fields.Status,
				UploadedAt: tt.fields.UploadedAt,
			}
			got, err := o.MarshalJSON()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, got, tt.want)
			}
		})
	}
}
