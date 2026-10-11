package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadRuleSet reads and validates a YAML rules configuration.
func LoadRuleSet(path string) (RuleSet, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- path is validated by validateConfigPath before use
	if err != nil {
		return RuleSet{}, fmt.Errorf("read config: %w", err)
	}

	if err := validateOptionalBooleanField(data, "consoleColorsEnabled"); err != nil {
		return RuleSet{}, err
	}
	if err := validateOptionalStringField(data, "baseline"); err != nil {
		return RuleSet{}, err
	}

	var ruleSet RuleSet
	if err := yaml.Unmarshal(data, &ruleSet); err != nil {
		return RuleSet{}, fmt.Errorf("parse config: %w", err)
	}

	if err := validateRuleSet(ruleSet); err != nil {
		return RuleSet{}, err
	}

	return ruleSet, nil
}

func validateOptionalBooleanField(data []byte, fieldName string) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	value, exists := raw[fieldName]
	if !exists {
		return nil
	}

	if _, ok := value.(bool); !ok {
		return fmt.Errorf("%s must be a boolean", fieldName)
	}

	return nil
}

func validateOptionalStringField(data []byte, fieldName string) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	value, exists := raw[fieldName]
	if !exists {
		return nil
	}

	if _, ok := value.(string); !ok {
		return fmt.Errorf("%s must be a string", fieldName)
	}

	return nil
}

func validateRequiredRules(ruleSet RuleSet) error {
	if len(ruleSet.Rules) == 0 && len(ruleSet.RuleFiles) == 0 {
		return fmt.Errorf("rules must be a non-empty list")
	}

	return nil
}

func validateRuleFilesEntries(entries StringList) error {
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			return fmt.Errorf("ruleFiles entries must be non-empty strings")
		}
		if _, ok := seen[entry]; ok {
			return fmt.Errorf("duplicate ruleFiles entry: %s", entry)
		}
		seen[entry] = struct{}{}
	}

	return nil
}

func validateRuleSetSettings(ruleSet RuleSet) error {
	if ruleSet.FailOn != nil && !isValidSeverity(*ruleSet.FailOn) {
		return fmt.Errorf("invalid failOn value: %s", *ruleSet.FailOn)
	}
	if ruleSet.Concurrency != nil && *ruleSet.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}
	if ruleSet.Baseline != nil && strings.TrimSpace(*ruleSet.Baseline) == "" {
		return fmt.Errorf("baseline must be a non-empty string")
	}

	return nil
}

func validateRuleSet(ruleSet RuleSet) error {
	if err := validateRuleSetFields(ruleSet); err != nil {
		return err
	}

	return validateRules(ruleSet.Rules)
}

func validateRuleSetFields(ruleSet RuleSet) error {
	if err := validateRequiredRules(ruleSet); err != nil {
		return err
	}
	if err := validateRuleFilesEntries(ruleSet.RuleFiles); err != nil {
		return err
	}
	if err := validateRuleSetSettings(ruleSet); err != nil {
		return err
	}
	if err := validateGitSettings(ruleSet.Git); err != nil {
		return err
	}
	if err := validateIgnoreFiles(ruleSet); err != nil {
		return err
	}

	return nil
}

func validateGitSettings(git *GitSettings) error {
	if git == nil {
		return nil
	}

	mode, err := validateGitMode(git.Mode)
	if err != nil {
		return err
	}
	if err := validateGitDiff(mode, git.Diff); err != nil {
		return err
	}
	if err := validateGitAddedLinesOnly(mode, git.AddedLinesOnly); err != nil {
		return err
	}

	return nil
}

func validateGitMode(modeValue *string) (string, error) {
	mode := "off"
	if modeValue == nil {
		return mode, nil
	}

	mode = *modeValue
	if mode == "off" || mode == "staged" || mode == "diff" {
		return mode, nil
	}

	return "", fmt.Errorf("git.mode must be one of off, staged, diff")
}

func validateGitDiff(mode string, diffValue *string) error {
	if diffValue != nil {
		if strings.TrimSpace(*diffValue) == "" {
			return fmt.Errorf("git.diff must be a non-empty string")
		}
		if mode != "diff" {
			return fmt.Errorf("git.diff is valid only when git.mode=diff")
		}
	}

	if mode == "diff" && diffValue == nil {
		return fmt.Errorf("git.mode=diff requires git.diff")
	}

	return nil
}

func validateGitAddedLinesOnly(mode string, addedLinesOnly *bool) error {
	if addedLinesOnly == nil || !*addedLinesOnly {
		return nil
	}
	if mode == "staged" || mode == "diff" {
		return nil
	}

	return fmt.Errorf("git.addedLinesOnly=true is valid only when git.mode=staged|diff")
}

func validateRules(rules []Rule) error {
	for i, rule := range rules {
		if err := validateRule(rule, i); err != nil {
			return err
		}
	}

	return nil
}

func validateRule(rule Rule, index int) error {
	if strings.TrimSpace(rule.Message) == "" {
		return fmt.Errorf("rule %d message is required", index+1)
	}
	if strings.TrimSpace(rule.Regex) == "" {
		return fmt.Errorf("rule %d regex is required", index+1)
	}
	if err := validateStringList(rule.Paths, "paths", index); err != nil {
		return err
	}
	if err := validateStringList(rule.Exclude, "exclude", index); err != nil {
		return err
	}
	if _, err := regexp.Compile(rule.Regex); err != nil {
		return fmt.Errorf("rule %d regex is invalid: %w", index+1, err)
	}
	if rule.Severity != "" && !isValidSeverity(rule.Severity) {
		return fmt.Errorf("rule %d has invalid severity: %s", index+1, rule.Severity)
	}

	return nil
}

func isValidSeverity(value string) bool {
	switch value {
	case "error", "warning", "notice", "info":
		return true
	default:
		return false
	}
}

func validateStringList(values []string, field string, index int) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("rule %d %s must not contain empty values", index+1, field)
		}
	}

	return nil
}

func validateIgnoreFiles(ruleSet RuleSet) error {
	seen := map[string]struct{}{}
	for _, value := range ruleSet.IgnoreFiles {
		name := strings.TrimSpace(value)
		if name == "" {
			return fmt.Errorf("ignoreFiles must not contain empty values")
		}
		if strings.ContainsAny(name, "/\\") {
			return fmt.Errorf("ignoreFiles must contain file names only")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("ignoreFiles must not contain duplicates")
		}
		seen[name] = struct{}{}
	}

	return nil
}
