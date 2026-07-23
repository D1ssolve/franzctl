package record

import (
	"reflect"
	"testing"
)

func TestValue(t *testing.T) {
	t.Parallel()
	if got := Value([]byte(`{"n":1}`)); !reflect.DeepEqual(got, map[string]any{"n": float64(1)}) {
		t.Fatalf("JSON value = %#v", got)
	}
	if got := Value([]byte("hello")); got != "hello" {
		t.Fatalf("string value = %#v", got)
	}
	if got, ok := Value([]byte{0xff}).(Binary); !ok || got.Encoding != "base64" || got.Data != "/w==" {
		t.Fatalf("binary value = %#v", got)
	}
}
