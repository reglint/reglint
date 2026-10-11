// Package config loads and validates rule configuration.
package config

// RuleSet represents the top-level YAML configuration.
type RuleSet struct {
	Rules                []Rule       `yaml:"rules"`
	Include              StringList   `yaml:"include,omitempty"`
	Exclude              StringList   `yaml:"exclude,omitempty"`
	FailOn               *string      `yaml:"failOn,omitempty"`
	Concurrency          *int         `yaml:"concurrency,omitempty"`
	Baseline             *string      `yaml:"baseline,omitempty"`
	Git                  *GitSettings `yaml:"git,omitempty"`
	RuleFiles            StringList   `yaml:"ruleFiles,omitempty"`
	ConsoleColorsEnabled *bool        `yaml:"consoleColorsEnabled,omitempty"`
	IgnoreFilesEnabled   *bool        `yaml:"ignoreFilesEnabled,omitempty"`
	IgnoreFiles          StringList   `yaml:"ignoreFiles,omitempty"`
}

// GitSettings represents optional RuleSet-level Git controls.
type GitSettings struct {
	Mode             *string `yaml:"mode,omitempty"`
	Diff             *string `yaml:"diff,omitempty"`
	AddedLinesOnly   *bool   `yaml:"addedLinesOnly,omitempty"`
	GitignoreEnabled *bool   `yaml:"gitignoreEnabled,omitempty"`
}
