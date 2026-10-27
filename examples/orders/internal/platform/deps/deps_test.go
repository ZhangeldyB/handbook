package deps

import (
	"io"
	"testing"
)

type thing struct{}

func TestRequire(t *testing.T) {
	var nilPointer *thing
	var nilReader io.Reader
	var typedNilInInterface io.Reader = (*nilReaderImpl)(nil)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "untyped nil", value: nil, wantErr: true},
		{name: "nil interface variable", value: nilReader, wantErr: true},
		{name: "nil pointer", value: nilPointer, wantErr: true},
		{name: "typed nil inside interface", value: typedNilInInterface, wantErr: true},
		{name: "nil func", value: (func())(nil), wantErr: true},
		{name: "non-nil pointer", value: &thing{}, wantErr: false},
		{name: "value type", value: thing{}, wantErr: false},
		{name: "non-nil func", value: func() {}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Require("Dep", tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Require() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type nilReaderImpl struct{}

func (*nilReaderImpl) Read([]byte) (int, error) { return 0, io.EOF }
