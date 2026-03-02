package requireenv

import (
	"os"
	"testing"
)

func TestLazy_DefersUntilCalled(t *testing.T) {
	f := Lazy("REQUIREENV_TEST_LAZY_MISSING")
	// should not have panicked yet
	_ = f
}

func TestLazy_ReturnsCachedValue(t *testing.T) {
	os.Setenv("REQUIREENV_TEST_LAZY", "hello")
	defer os.Unsetenv("REQUIREENV_TEST_LAZY")

	f := Lazy("REQUIREENV_TEST_LAZY")
	if got := f(); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	// second call returns same value
	if got := f(); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestLazy_PanicsWhenMissing(t *testing.T) {
	f := Lazy("REQUIREENV_TEST_LAZY_MISSING")
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic, got none")
		}
	}()
	f()
}
