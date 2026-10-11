package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reglint/reglint/internal/config"
)

const ruleFilesBaseRules = "rules:\n  - message: \"Base\"\n    regex: \"base=\"\n"
const ruleFilesPackRules = "rules:\n  - message: \"Found token\"\n    regex: \"token=\"\n"
const ruleFilesExtraRules = "rules:\n  - message: \"Extra\"\n    regex: \"extra=\"\n"

var loadRuleSetWithFilesCases = []struct {
	name       string
	configPath string
	files      map[string]string
	extras     []string
	wantErr    string
	wantRules  []string
}{
	{
		name:       "base rules followed by ruleFiles rules",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    ruleFilesPackRules,
		},
		wantRules: []string{"Base", "Found token"},
	},
	{
		name:       "empty base rules with non-empty ruleFiles loads",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    ruleFilesPackRules,
		},
		wantRules: []string{"Found token"},
	},
	{
		name:       "no rules and no ruleFiles errors",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": "failOn: warning\n",
		},
		wantErr: "rules must be a non-empty list",
	},
	{
		name:       "empty ruleFiles entry errors",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - \"\"\n",
		},
		wantErr: "ruleFiles entries must be non-empty strings",
	},
	{
		name:       "duplicate raw ruleFiles entry errors",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n  - pack.yaml\n",
			"pack.yaml":    ruleFilesPackRules,
		},
		wantErr: "duplicate ruleFiles entry: pack.yaml",
	},
	{
		name:       "ruleFiles entry resolves relative to config dir",
		configPath: filepath.Join("conf", "reglint.yaml"),
		files: map[string]string{
			filepath.Join("conf", "reglint.yaml"): ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			filepath.Join("conf", "pack.yaml"):    ruleFilesPackRules,
		},
		wantRules: []string{"Base", "Found token"},
	},
	{
		name:       "rule file with failOn errors naming the key",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    ruleFilesPackRules + "failOn: error\n",
		},
		wantErr: "only \"rules\" is allowed in rule files, found \"failOn\"",
	},
	{
		name:       "rule file with unknown key errors naming the key",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    ruleFilesPackRules + "bogus: 1\n",
		},
		wantErr: "only \"rules\" is allowed in rule files, found \"bogus\"",
	},
	{
		name:       "rule file with empty rules errors",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    "rules: []\n",
		},
		wantErr: "rules must be a non-empty list",
	},
	{
		name:       "rule file with invalid rule errors with validateRules message",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    "rules:\n  - message: \"Bad\"\n    regex: \"(unclosed\"\n",
		},
		wantErr: "rule 1 regex is invalid",
	},
	{
		name:       "duplicate rule base vs rule file errors naming both sources",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": "rules:\n  - message: \"Dup\"\n    regex: \"dup=\"\nruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    "rules:\n  - message: \"Dup\"\n    regex: \"dup=\"\n",
		},
		wantErr: "duplicate rule (message \"Dup\", regex \"dup=\"): first defined in",
	},
	{
		name:       "duplicate rule within one rule file errors",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml": "rules:\n" +
				"  - message: \"Dup\"\n    regex: \"dup=\"\n" +
				"  - message: \"Dup\"\n    regex: \"dup=\"\n",
		},
		wantErr: "duplicate rule (message \"Dup\", regex \"dup=\"): first defined in",
	},
	{
		name:       "same resolved path via ruleFiles and extra errors",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    ruleFilesPackRules,
		},
		extras:  []string{"pack.yaml"},
		wantErr: "duplicate rule file path:",
	},
	{
		name:       "missing rule file errors",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - missing.yaml\n",
		},
		wantErr: "rule file not found:",
	},
	{
		name:       "extra rule files load after config ruleFiles",
		configPath: "reglint.yaml",
		files: map[string]string{
			"reglint.yaml": ruleFilesBaseRules + "ruleFiles:\n  - pack.yaml\n",
			"pack.yaml":    ruleFilesPackRules,
			"extra.yaml":   ruleFilesExtraRules,
		},
		extras:    []string{"extra.yaml"},
		wantRules: []string{"Base", "Found token", "Extra"},
	},
}

func TestLoadRuleSetWithFiles(t *testing.T) {
	t.Parallel()

	for _, tt := range loadRuleSetWithFilesCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeRuleFilesFixtures(t, dir, tt.files)

			extras := make([]string, 0, len(tt.extras))
			for _, extra := range tt.extras {
				extras = append(extras, filepath.Join(dir, extra))
			}

			got, err := config.LoadRuleSetWithFiles(filepath.Join(dir, tt.configPath), extras)

			if tt.wantErr != "" {
				assertRuleFilesError(t, err, tt.wantErr)

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertMergedRules(t, got, tt.wantRules)
		})
	}
}

func writeRuleFilesFixtures(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create parent dirs: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
}

func assertRuleFilesError(t *testing.T, err error, wantErr string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q, got nil", wantErr)
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("expected error containing %q, got %v", wantErr, err)
	}
}

func assertMergedRules(t *testing.T, got config.RuleSet, wantRules []string) {
	t.Helper()

	if got.RuleFiles != nil {
		t.Errorf("expected RuleFiles nil after merge, got %v", got.RuleFiles)
	}
	if len(got.Rules) != len(wantRules) {
		t.Fatalf("expected %d rules, got %d: %v", len(wantRules), len(got.Rules), got.Rules)
	}
	for i, want := range wantRules {
		if got.Rules[i].Message != want {
			t.Errorf("rule %d: expected message %q, got %q", i, want, got.Rules[i].Message)
		}
	}
}
