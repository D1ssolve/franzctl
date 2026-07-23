package app

import "testing"

func TestParseOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		kind  offsetKind
		value int64
	}{
		{input: "beginning", kind: offsetBeginning},
		{input: "end", kind: offsetEnd},
		{input: "offset:42", kind: offsetAbsolute, value: 42},
	}
	for _, test := range tests {
		got, err := parseOffset(test.input)
		if err != nil {
			t.Fatalf("parseOffset(%q): %v", test.input, err)
		}
		if got.Kind != test.kind || got.Value != test.value {
			t.Fatalf("parseOffset(%q) = %#v", test.input, got)
		}
	}
	if _, err := parseOffset("offset:-1"); err == nil {
		t.Fatal("expected negative offset to fail")
	}
}
