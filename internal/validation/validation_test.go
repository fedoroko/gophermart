package validation

import "testing"

func TestIsValid(t *testing.T) {
	type args struct {
		number int64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "positive #1",
			args: args{
				number: 2375460850,
			},
			want: true,
		},
		{
			name: "positive #2",
			args: args{
				number: 5512703182881200,
			},
			want: true,
		},
		{
			name: "negative #1",
			args: args{
				number: 2375460851,
			},
			want: false,
		},
		{
			name: "negative #2",
			args: args{
				number: 5512703182881204,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.args.number); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
