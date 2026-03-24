package validation

import (
	"errors"
	"testing"
)

func TestFormatValidValues(t *testing.T) {
	type sample string

	const (
		first  sample = "first"
		second sample = "second"
	)

	got := FormatValidValues([]sample{first, second})
	want := "first, second"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestIsValidValue(t *testing.T) {
	type sample string

	const (
		first  sample = "first"
		second sample = "second"
	)

	if !IsValidValue(first, []sample{first, second}) {
		t.Fatalf("expected value to be valid")
	}

	if IsValidValue(sample("third"), []sample{first, second}) {
		t.Fatalf("expected value to be invalid")
	}
}

func TestFormatInvalidValueError(t *testing.T) {
	type sample string

	const (
		first  sample = "first"
		second sample = "second"
	)

	base := errors.New("invalid sample")
	err := FormatInvalidValueError(base, sample("bad"), []sample{first, second})
	if !errors.Is(err, base) {
		t.Fatalf("expected error to wrap %v", base)
	}

	want := "invalid sample: \"bad\" (valid: first, second)"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
}
