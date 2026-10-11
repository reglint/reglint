package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/reglint/reglint/internal/config"
)

// ValidateConfig holds parsed validate command inputs.
type ValidateConfig struct {
	ConfigPath    string
	RuleFilePaths []string
}

// HandleValidate executes the validate command: checks a config and its
// merged rules without scanning any files.
func HandleValidate(args []string, out *bytes.Buffer) int {
	cfg, err := ParseValidateArgs(args)
	if err != nil {
		writeError(out, err)

		return exitCodeError
	}

	if err := validateConfigPath(cfg.ConfigPath); err != nil {
		writeError(out, err)

		return exitCodeError
	}

	ruleSet, err := config.LoadRuleSetWithFiles(cfg.ConfigPath, cfg.RuleFilePaths)
	if err != nil {
		writeError(out, err)

		return exitCodeError
	}

	_, _ = fmt.Fprintf(out, "Config OK: %s (%d rules)\n", cfg.ConfigPath, len(ruleSet.Rules))

	return 0
}

// ParseValidateArgs parses validate command arguments into a config.
func ParseValidateArgs(args []string) (ValidateConfig, error) {
	var cfg ValidateConfig

	flagSet := flag.NewFlagSet("validate", flag.ContinueOnError)
	flagSet.SetOutput(&strings.Builder{})

	flagSet.StringVar(&cfg.ConfigPath, "config", defaultConfigPath, "Path to YAML rules config file.")
	flagSet.StringVar(&cfg.ConfigPath, "c", defaultConfigPath, "Path to YAML rules config file.")
	ruleFiles := &stringSlice{}
	flagSet.Var(ruleFiles, "rules", "Repeatable rule file path loaded after config ruleFiles.")

	if err := flagSet.Parse(args); err != nil {
		return ValidateConfig{}, err
	}

	if strings.TrimSpace(cfg.ConfigPath) == "" {
		return ValidateConfig{}, errors.New("config path must not be empty")
	}

	if len(*ruleFiles) > 0 {
		cfg.RuleFilePaths = []string(*ruleFiles)
	}

	return cfg, nil
}
