//nolint:testpackage
package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/scan"
)

func TestConsoleFormatterNameAndWrite(t *testing.T) {
	t.Parallel()

	formatter := ConsoleFormatter{}
	if formatter.Name() != "console" {
		t.Fatalf("unexpected formatter name: %s", formatter.Name())
	}

	var buffer bytes.Buffer
	if err := formatter.Write(scan.Result{}, &buffer); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if !strings.Contains(buffer.String(), "No matches found.") {
		t.Fatalf("unexpected console output: %q", buffer.String())
	}
}

func TestJSONFormatterNameAndWrite(t *testing.T) {
	t.Parallel()

	formatter := JSONFormatter{}
	if formatter.Name() != "json" {
		t.Fatalf("unexpected formatter name: %s", formatter.Name())
	}

	var buffer bytes.Buffer
	if err := formatter.Write(scan.Result{}, &buffer); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	var payload struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("unexpected schema version: %d", payload.SchemaVersion)
	}
}

func TestSARIFFormatterNameAndWrite(t *testing.T) {
	t.Parallel()

	formatter := SARIFFormatter{}
	if formatter.Name() != "sarif" {
		t.Fatalf("unexpected formatter name: %s", formatter.Name())
	}

	var buffer bytes.Buffer
	if err := formatter.Write(scan.Result{}, &buffer); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if !strings.Contains(buffer.String(), "RegLint") {
		t.Fatalf("unexpected sarif output: %q", buffer.String())
	}
}
