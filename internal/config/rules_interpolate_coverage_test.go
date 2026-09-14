package config_test

import (
	"testing"

	"github.com/reglint/reglint/internal/rules"
)

func TestInterpolateMessageFromConfigPackageCoverage(t *testing.T) {
	t.Parallel()
	runInterpolationCase(t, "empty message", "", []string{"m0"}, "")
	runInterpolationCase(t, "plain text", "no substitutions", []string{"m0"}, "no substitutions")
	runInterpolationCase(t, "full match substitution", "value=$0", []string{"token=abc"}, "value=token=abc")
	runInterpolationCase(t, "single digit group", "group=$1", []string{"token=abc", "abc"}, "group=abc")
	runInterpolationCase(
		t,
		"multi digit group",
		"group=$10",
		[]string{"m0", "m1", "m2", "m3", "m4", "m5", "m6", "m7", "m8", "m9", "m10"},
		"group=m10",
	)
	runInterpolationCase(t, "out of range group", "group=$12", []string{"m0", "m1"}, "group=")
	runInterpolationCase(t, "escaped dollar", "price=$$5", []string{"m0"}, "price=$5")
	runInterpolationCase(
		t,
		"non digit after dollar",
		"currency=$USD",
		[]string{"m0"},
		"currency=$USD",
	)
	runInterpolationCase(t, "trailing dollar", "currency=$", []string{"m0"}, "currency=$")
	runInterpolationCase(
		t,
		"digit parsing stops at non digit",
		"group=$10x",
		[]string{"m0", "m1", "m2", "m3", "m4", "m5", "m6", "m7", "m8", "m9", "m10"},
		"group=m10x",
	)
}

func runInterpolationCase(t *testing.T, name, message string, captures []string, want string) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		t.Parallel()

		got := rules.InterpolateMessage(message, captures)
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}
