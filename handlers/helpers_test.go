package handlers

import (
	"testing"
)

func TestNormalizeExtra_nil(t *testing.T) {
	result := normalizeExtra(nil)
	if string(result) != "{}" {
		t.Fatalf("expected {}, got %s", result)
	}
}

func TestNormalizeExtra_empty(t *testing.T) {
	result := normalizeExtra([]byte{})
	if string(result) != "{}" {
		t.Fatalf("expected {}, got %s", result)
	}
}

func TestNormalizeExtra_passthrough(t *testing.T) {
	input := []byte(`{"foo":"bar"}`)
	result := normalizeExtra(input)
	if string(result) != string(input) {
		t.Fatalf("expected %s, got %s", input, result)
	}
}

func TestPqStringArray_roundtrip(t *testing.T) {
	cases := []struct {
		input    []string
		expected []string
	}{
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{}, []string{}},
		{[]string{"has space", `has"quote`}, []string{"has space", `has"quote`}},
	}

	for _, tc := range cases {
		arr := pqStringArray(tc.input)
		val, err := arr.Value()
		if err != nil {
			t.Fatalf("Value() error: %v", err)
		}

		var out pqStringArray
		if err := out.Scan(val); err != nil {
			t.Fatalf("Scan() error: %v", err)
		}

		if len(out) != len(tc.expected) {
			t.Fatalf("length mismatch: got %d, want %d", len(out), len(tc.expected))
		}
		for i := range tc.expected {
			if out[i] != tc.expected[i] {
				t.Fatalf("element %d: got %q, want %q", i, out[i], tc.expected[i])
			}
		}
	}
}

func TestPqStringArray_nilValue(t *testing.T) {
	var arr pqStringArray
	val, err := arr.Value()
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil value for nil array, got %v", val)
	}
}

func TestPqStringArray_scanNil(t *testing.T) {
	var arr pqStringArray
	if err := arr.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if arr != nil {
		t.Fatalf("expected nil after scanning nil, got %v", arr)
	}
}
