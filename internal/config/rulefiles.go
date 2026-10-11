package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// LoadRuleSetWithFiles loads the base config, then merges rules from its
// ruleFiles entries followed by extraRuleFiles (CLI --rules), in order.
func LoadRuleSetWithFiles(configPath string, extraRuleFiles []string) (RuleSet, error) {
	ruleSet, err := LoadRuleSet(configPath)
	if err != nil {
		return RuleSet{}, err
	}

	sources, err := resolveRuleFileSources(configPath, ruleSet.RuleFiles, extraRuleFiles)
	if err != nil {
		return RuleSet{}, err
	}

	seen := make(map[ruleKey]string, len(ruleSet.Rules))
	merged := make([]Rule, len(ruleSet.Rules))
	copy(merged, ruleSet.Rules)
	for _, rule := range ruleSet.Rules {
		seen[ruleKey{message: rule.Message, regex: rule.Regex}] = configPath
	}

	for _, path := range sources {
		fileRules, err := loadRuleFile(path)
		if err != nil {
			return RuleSet{}, err
		}
		for _, rule := range fileRules {
			key := ruleKey{message: rule.Message, regex: rule.Regex}
			if first, ok := seen[key]; ok {
				return RuleSet{}, fmt.Errorf(
					"duplicate rule (message %q, regex %q): first defined in %s, redefined in %s",
					rule.Message, rule.Regex, first, path,
				)
			}
			seen[key] = path
			merged = append(merged, rule)
		}
	}

	ruleSet.Rules = merged
	ruleSet.RuleFiles = nil

	return ruleSet, nil
}

// resolveRuleFileSources maps config ruleFiles entries relative to the config
// directory, appends extraRuleFiles verbatim, and rejects resolved duplicates.
func resolveRuleFileSources(configPath string, entries StringList, extraRuleFiles []string) ([]string, error) {
	sources := make([]string, 0, len(entries)+len(extraRuleFiles))
	for _, entry := range entries {
		path := entry
		if !filepath.IsAbs(entry) {
			path = filepath.Join(filepath.Dir(configPath), entry)
		}
		sources = append(sources, path)
	}
	sources = append(sources, extraRuleFiles...)

	seenPaths := make(map[string]struct{}, len(sources))
	for _, path := range sources {
		if _, ok := seenPaths[path]; ok {
			return nil, fmt.Errorf("duplicate rule file path: %s", path)
		}
		seenPaths[path] = struct{}{}
	}

	return sources, nil
}

type ruleKey struct {
	message string
	regex   string
}

// validateRuleFileKeys rejects every top-level key other than "rules",
// reporting the first offender in sorted order for deterministic errors.
func validateRuleFileKeys(raw map[string]any, path string) error {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key != "rules" {
			return fmt.Errorf("rule file %s: only \"rules\" is allowed in rule files, found %q", path, key)
		}
	}

	return nil
}

func loadRuleFile(path string) ([]Rule, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("rule file not found: %s", path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("rule file path is a directory: %s", path)
	}

	data, err := os.ReadFile(path) //#nosec G304 -- path comes from config or CLI input
	if err != nil {
		return nil, fmt.Errorf("rule file not readable: %s", path)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse rule file %s: %w", path, err)
	}
	if err := validateRuleFileKeys(raw, path); err != nil {
		return nil, err
	}

	var ruleFile RuleSet
	if err := yaml.Unmarshal(data, &ruleFile); err != nil {
		return nil, fmt.Errorf("parse rule file %s: %w", path, err)
	}
	if err := validateRules(ruleFile.Rules); err != nil {
		return nil, fmt.Errorf("rule file %s: %w", path, err)
	}
	if len(ruleFile.Rules) == 0 {
		return nil, fmt.Errorf("rule file %s: rules must be a non-empty list", path)
	}

	return ruleFile.Rules, nil
}
