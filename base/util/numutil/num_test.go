package numutil

import (
	"github.com/samber/lo"
	"testing"
)

/**
  *  @author tryao
  *  @date 2022/03/23 15:28
**/

func TestConvertToBool(t *testing.T) {
	type args struct {
		unk any
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"bool", args{true}, true, false},
		{"0", args{0}, false, false},
		{"1", args{1}, true, false},
		{"string", args{"string"}, true, false},
		{"empty", args{""}, false, false},
		{"nil", args{nil}, false, false},
		{"pointer", args{lo.ToPtr("test")}, true, false},
		{"slice", args{[]int{1, 2, 3}}, true, false},
		{"empty map", args{map[int]bool{}}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToBool(tt.args.unk)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertToBool() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToInt64(t *testing.T) {
	type args struct {
		temp any
	}
	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr bool
	}{
		{"0", args{0}, 0, false},
		{"string", args{"0123"}, 123, false},
		{"float", args{1.234}, 1, false},
		{"float string", args{"0.55"}, 0, false},
		{"bool", args{true}, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToInt64(tt.args.temp)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToInt64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertToInt64() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToFloat64(t *testing.T) {
	type args struct {
		unk any
	}
	tests := []struct {
		name    string
		args    args
		want    float64
		wantErr bool
	}{
		{"0", args{0}, 0, false},
		{"string", args{"0.123"}, 0.123, false},
		{"1.23", args{1.23}, 1.23, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToFloat64(tt.args.unk)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToFloat64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertToFloat64() got = %v, want %v", got, tt.want)
			}
		})
	}
}
