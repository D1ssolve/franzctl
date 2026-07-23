package codec

import (
	"bytes"
	"context"
	"testing"
)

func TestRoundTrips(t *testing.T) {
	t.Parallel()
	tests := []struct {
		spec  string
		input []byte
	}{
		{spec: "bytes", input: []byte("hello")},
		{spec: "string", input: []byte("Привет")},
		{spec: "json", input: []byte(`{"hello": "world"}`)},
		{spec: "base64", input: []byte("aGVsbG8=")},
		{spec: "hex", input: []byte("00ff10")},
		{spec: "int64", input: []byte("-9223372036854775808")},
		{spec: "uint64", input: []byte("18446744073709551615")},
		{spec: "float64", input: []byte("3.141592653589793")},
		{spec: "bool", input: []byte("true")},
		{spec: "json,gzip", input: []byte(`{"hello":"world"}`)},
	}
	for _, test := range tests {
		test := test
		t.Run(test.spec, func(t *testing.T) {
			t.Parallel()
			pipeline, err := Parse(test.spec)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := pipeline.Encode(context.Background(), test.input)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := pipeline.Decode(context.Background(), encoded)
			if err != nil {
				t.Fatal(err)
			}

			expected := test.input
			if test.spec == "json" {
				expected = []byte(`{"hello":"world"}`)
			}
			if !bytes.Equal(decoded, expected) {
				t.Fatalf("round trip mismatch: got %q, want %q", decoded, expected)
			}
		})
	}
}

func TestParseRejectsUnknownCodec(t *testing.T) {
	t.Parallel()
	if _, err := Parse("missing"); err == nil {
		t.Fatal("expected an error")
	}
}
