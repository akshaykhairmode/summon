package main

import (
	"bytes"
	"testing"
)

func Test_encode(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		wantW   string
		wantErr bool
	}{
		{"Valid Case 1", args{"TEST"}, "VEVTVA==", false},
		{"Valid Case 2", args{"0_12345_12345"}, "MF8xMjM0NV8xMjM0NQ==", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			if err := encode(tt.args.s, w); (err != nil) != tt.wantErr {
				t.Errorf("encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("encode() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}

func Test_decode(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"Valid Case 1", args{"VEVTVA=="}, "TEST", false},
		{"Valid Case 2", args{"MF8xMjM0NV8xMjM0NQ=="}, "0_12345_12345", false},
		{"Invalid Case 1", args{"asd"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decode(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("decode() = %v, want %v", got, tt.want)
			}
		})
	}
}
