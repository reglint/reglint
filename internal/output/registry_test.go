//nolint:testpackage
package output

import (
	"io"
	"testing"

	"github.com/reglint/reglint/internal/scan"
)

type fakeFormatter struct {
	name string
}

func (f fakeFormatter) Name() string {
	return f.name
}

func (f fakeFormatter) Write(_ scan.Result, _ io.Writer) error {
	return nil
}

func TestRegistryResolvesFormatsInOrder(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(
		fakeFormatter{name: "console"},
		fakeFormatter{name: "json"},
	)
	if err != nil {
		t.Fatalf("unexpected registry error: %v", err)
	}

	resolved, err := registry.Resolve([]string{"json", "console"})
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2 formatters, got %d", len(resolved))
	}
	if resolved[0].Name() != "json" || resolved[1].Name() != "console" {
		t.Fatalf("unexpected format order: %s, %s", resolved[0].Name(), resolved[1].Name())
	}
}

func TestRegistryRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(fakeFormatter{name: "console"})
	if err != nil {
		t.Fatalf("unexpected registry error: %v", err)
	}

	if _, err := registry.Resolve([]string{"missing"}); err == nil {
		t.Fatalf("expected resolve error, got nil")
	}
}

func TestRegistryRejectsDuplicateFormats(t *testing.T) {
	t.Parallel()

	if _, err := NewRegistry(
		fakeFormatter{name: "console"},
		fakeFormatter{name: "console"},
	); err == nil {
		t.Fatalf("expected duplicate format error, got nil")
	}
}

func TestRegistryResolvesName(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(fakeFormatter{name: "console"})
	if err != nil {
		t.Fatalf("unexpected registry error: %v", err)
	}

	formatter, err := registry.ResolveName("console")
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if formatter.Name() != "console" {
		t.Fatalf("unexpected formatter name: %s", formatter.Name())
	}
}

func TestRegistryResolveNameRejectsUnknown(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(fakeFormatter{name: "console"})
	if err != nil {
		t.Fatalf("unexpected registry error: %v", err)
	}

	if _, err := registry.ResolveName("missing"); err == nil {
		t.Fatalf("expected resolve error, got nil")
	}
}

func TestRegistryRejectsUppercaseFormatName(t *testing.T) {
	t.Parallel()

	if _, err := NewRegistry(fakeFormatter{name: "Console"}); err == nil {
		t.Fatalf("expected lowercase format error, got nil")
	}
}

func TestRegistryRejectsNilFormatter(t *testing.T) {
	t.Parallel()

	var formatter Formatter
	if _, err := NewRegistry(formatter); err == nil {
		t.Fatalf("expected nil formatter error, got nil")
	}
}

func TestRegistryRejectsEmptyFormatterName(t *testing.T) {
	t.Parallel()

	if _, err := NewRegistry(fakeFormatter{name: "  "}); err == nil {
		t.Fatalf("expected empty formatter name error, got nil")
	}
}
