package config

import (
	"runtime"

	"github.com/reglint/reglint/internal/rules"
)

// Rule represents a single regex rule entry.
type Rule struct {
	Message  string     `yaml:"message"`
	Regex    string     `yaml:"regex"`
	Severity string     `yaml:"severity,omitempty"`
	Paths    StringList `yaml:"paths,omitempty"`
	Exclude  StringList `yaml:"exclude,omitempty"`
}

func (r Rule) toRulesRule() rules.Rule {
	severity := r.Severity
	if severity == "" {
		severity = "warning"
	}

	paths := append([]string{}, []string(r.Paths)...)
	if len(paths) == 0 {
		paths = []string{"**/*"}
	}

	exclude := append([]string{}, []string(r.Exclude)...)

	return rules.Rule{
		Message:  r.Message,
		Regex:    r.Regex,
		Severity: severity,
		Paths:    paths,
		Exclude:  exclude,
	}
}

// ToRules converts the parsed config into shared rule models.
func (ruleSet RuleSet) ToRules() rules.RuleSet {
	converted := rules.RuleSet{
		Include:              append([]string{}, []string(ruleSet.Include)...),
		Exclude:              append([]string{}, []string(ruleSet.Exclude)...),
		FailOn:               ruleSet.FailOn,
		Concurrency:          ruleSet.Concurrency,
		Baseline:             copyStringPointer(ruleSet.Baseline),
		Git:                  toRulesGitSettings(ruleSet.Git),
		ConsoleColorsEnabled: ruleSet.ConsoleColorsEnabled,
		IgnoreFilesEnabled:   ruleSet.IgnoreFilesEnabled,
		IgnoreFiles:          append([]string{}, []string(ruleSet.IgnoreFiles)...),
	}

	applyRuleSetDefaults(&converted)
	converted.Rules = toRulesRules(ruleSet.Rules, converted.Include, converted.Exclude)

	return converted
}

func copyStringPointer(value *string) *string {
	if value == nil {
		return nil
	}

	copyValue := *value

	return &copyValue
}

func toRulesGitSettings(git *GitSettings) rules.GitSettings {
	settings := rules.GitSettings{
		Mode:             "off",
		Diff:             "",
		AddedLinesOnly:   false,
		GitignoreEnabled: true,
	}

	if git == nil {
		return settings
	}
	if git.Mode != nil {
		settings.Mode = *git.Mode
	}
	if git.Diff != nil {
		settings.Diff = *git.Diff
	}
	if git.AddedLinesOnly != nil {
		settings.AddedLinesOnly = *git.AddedLinesOnly
	}
	if git.GitignoreEnabled != nil {
		settings.GitignoreEnabled = *git.GitignoreEnabled
	}

	return settings
}

func applyRuleSetDefaults(converted *rules.RuleSet) {
	if len(converted.Include) == 0 {
		converted.Include = []string{"**/*"}
	}
	if len(converted.Exclude) == 0 {
		converted.Exclude = []string{"**/.git/**", "**/node_modules/**", "**/vendor/**"}
	}
	if converted.Concurrency == nil {
		concurrency := runtime.GOMAXPROCS(0)
		converted.Concurrency = &concurrency
	}
}

func toRulesRules(configRules []Rule, include, exclude []string) []rules.Rule {
	if len(configRules) == 0 {
		return nil
	}

	convertedRules := make([]rules.Rule, len(configRules))
	for i, rule := range configRules {
		convertedRule := rule.toRulesRule()
		convertedRule.Index = i
		if len(rule.Paths) == 0 {
			convertedRule.Paths = append([]string{}, include...)
		}
		if len(rule.Exclude) == 0 {
			convertedRule.Exclude = append([]string{}, exclude...)
		}
		convertedRules[i] = convertedRule
	}

	return convertedRules
}
