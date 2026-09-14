package rules_test

import (
	"testing"

	"github.com/reglint/reglint/internal/rules"
)

func TestInterpolateMessage(t *testing.T) {
	t.Parallel()

	assertInterpolation(
		t,
		"replaces capture groups",
		"Avoid hardcoded token: $1",
		[]string{"token=abc123", "abc123"},
		"Avoid hardcoded token: abc123",
	)
	assertInterpolation(
		t,
		"replaces full match",
		"Full match: $0",
		[]string{"full"},
		"Full match: full",
	)
	assertInterpolation(
		t,
		"replaces multi-digit index",
		"Group $10",
		[]string{"m0", "m1", "m2", "m3", "m4", "m5", "m6", "m7", "m8", "m9", "m10"},
		"Group m10",
	)
	assertInterpolation(
		t,
		"missing capture resolves to empty",
		"Missing $3",
		[]string{"m0", "m1"},
		"Missing ",
	)
	assertInterpolation(
		t,
		"escapes literal dollar",
		"Total $$5",
		[]string{"m0"},
		"Total $5",
	)
	assertInterpolation(
		t,
		"treats non-digit as literal",
		"Cost $USD",
		[]string{"m0"},
		"Cost $USD",
	)
	assertInterpolation(
		t,
		"trailing dollar literal",
		"Value $",
		[]string{"m0"},
		"Value $",
	)
}

func assertInterpolation(t *testing.T, name, message string, captures []string, expected string) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		t.Parallel()

		got := rules.InterpolateMessage(message, captures)
		if got != expected {
			t.Fatalf("expected %q, got %q", expected, got)
		}
	})
}
