package resource

import (
	"math"
	"testing"
)

func TestStringOrEmptyHandlesDatabaseText(t *testing.T) {
	for _, test := range []struct {
		name  string
		value interface{}
		want  string
	}{
		{name: "string", value: "held", want: "held"},
		{name: "bytes", value: []byte("held"), want: "held"},
		{name: "nil", value: nil, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := StringOrEmpty(test.value); got != test.want {
				t.Fatalf("StringOrEmpty(%T) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestResourceRowBoolHandlesDatabaseRepresentations(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   interface{}
		want    bool
		wantErr bool
	}{
		{name: "boolean true", value: true, want: true},
		{name: "boolean false", value: false},
		{name: "int enabled", value: int(1), want: true},
		{name: "int disabled", value: int(0)},
		{name: "int64 enabled", value: int64(1), want: true},
		{name: "numeric string enabled", value: "1", want: true},
		{name: "string true enabled", value: "true", want: true},
		{name: "string false disabled", value: "false"},
		{name: "unsupported", value: struct{}{}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resourceRowBool(test.value)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("resourceRowBool(%T) = %v, %v; want %v, error=%v", test.value, got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestResourceRowInt64RejectsLossyConversion(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    int64
		wantErr bool
	}{
		{name: "int64", value: int64(7), want: 7},
		{name: "int", value: int(7), want: 7},
		{name: "int32", value: int32(7), want: 7},
		{name: "uint64", value: uint64(7), want: 7},
		{name: "bytes", value: []byte("7"), want: 7},
		{name: "string", value: "7", want: 7},
		{name: "uint64 overflow", value: uint64(math.MaxInt64) + 1, wantErr: true},
		{name: "invalid string", value: "invalid", wantErr: true},
		{name: "nil", value: nil, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResourceRowInt64(test.value)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("ResourceRowInt64(%T) = %d, %v; want %d, error=%v", test.value, got, err, test.want, test.wantErr)
			}
		})
	}
}
