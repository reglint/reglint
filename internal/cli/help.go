package cli

import (
	"fmt"
	"io"
	"strings"
)

// HelpTopic describes a CLI help topic.
type HelpTopic struct {
	Name    string
	Usage   []string
	Aliases []string
	Flags   []HelpFlag
}

// HelpFlag describes a CLI help flag.
type HelpFlag struct {
	Long        string
	Short       string
	Type        string
	Default     string
	Description string
}

func getHelpTopic(name string) (HelpTopic, bool) {
	switch name {
	case "root":
		return rootHelpTopic(), true
	case "analyse":
		return analyzeHelpTopic(), true
	case "analyze":
		return analyzeHelpTopic(), true
	case "init":
		return initHelpTopic(), true
	case "version":
		return versionHelpTopic(), true
	default:
		return HelpTopic{}, false
	}
}

func writeHelpTopic(out io.Writer, topic HelpTopic) {
	_, _ = fmt.Fprintln(out, "Usage:")
	for _, line := range topic.Usage {
		_, _ = fmt.Fprintf(out, "  %s\n", line)
	}

	if topic.Name == "root" {
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "Commands:")
		for _, command := range rootCommands() {
			_, _ = fmt.Fprintf(out, "  %s\n", command)
		}
	}

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Flags:")
	for _, flag := range topic.Flags {
		_, _ = fmt.Fprintln(out, formatHelpFlag(flag))
	}
}

func formatHelpFlag(flag HelpFlag) string {
	short := strings.TrimSpace(flag.Short)
	if short == "" {
		return fmt.Sprintf(
			"      %s %s (default %s)  %s",
			flag.Long,
			flag.Type,
			flag.Default,
			flag.Description,
		)
	}

	return fmt.Sprintf(
		"  %s, %s %s (default %s)  %s",
		short,
		flag.Long,
		flag.Type,
		flag.Default,
		flag.Description,
	)
}

func rootCommands() []string {
	return []string{
		"analyze (alias: analyse)",
		"init",
		"version",
	}
}

func rootHelpTopic() HelpTopic {
	return HelpTopic{
		Name:  "root",
		Usage: []string{"reglint <command> [flags]"},
		Flags: []HelpFlag{
			helpFlag(),
		},
	}
}

func analyzeHelpTopic() HelpTopic {
	return HelpTopic{
		Name:    "analyze",
		Usage:   []string{"reglint analyze [flags] [path ...]", "reglint analyse [flags] [path ...]"},
		Aliases: []string{"analyse"},
		Flags:   analyzeHelpFlags(),
	}
}

func analyzeHelpFlags() []HelpFlag {
	flags := analyzeHelpCoreFlags()
	flags = append(flags, analyzeHelpOutputFlags()...)
	flags = append(flags, analyzeHelpFilterFlags()...)
	flags = append(flags, analyzeHelpRuntimeFlags()...)
	flags = append(flags, analyzeHelpGitFlags()...)
	flags = append(flags, analyzeHelpIgnoreFlags()...)

	return flags
}

func analyzeHelpCoreFlags() []HelpFlag {
	return []HelpFlag{
		helpFlag(),
		{
			Long:        "--config",
			Short:       "-c",
			Type:        "string",
			Default:     defaultConfigPath,
			Description: "Path to YAML rules config file.",
		},
		{
			Long:        "--format",
			Short:       "-f",
			Type:        "string",
			Default:     defaultFormat,
			Description: "Comma-separated list of formats.",
		},
	}
}

func analyzeHelpOutputFlags() []HelpFlag {
	return []HelpFlag{
		{
			Long:        "--out-json",
			Type:        "string",
			Default:     "none",
			Description: "Output path for JSON results.",
		},
		{
			Long:        "--out-sarif",
			Type:        "string",
			Default:     "none",
			Description: "Output path for SARIF results.",
		},
	}
}

func analyzeHelpFilterFlags() []HelpFlag {
	return []HelpFlag{
		{
			Long:        "--include",
			Type:        "string",
			Default:     "none",
			Description: "Repeatable include glob.",
		},
		{
			Long:        "--exclude",
			Type:        "string",
			Default:     "none",
			Description: "Repeatable exclude glob.",
		},
	}
}

func analyzeHelpRuntimeFlags() []HelpFlag {
	return []HelpFlag{
		{
			Long:        "--concurrency",
			Type:        "int",
			Default:     "GOMAXPROCS",
			Description: "Worker count.",
		},
		{
			Long:        "--max-file-size",
			Type:        "int",
			Default:     fmt.Sprintf("%d", defaultMaxFileBytes),
			Description: "Maximum file size in bytes.",
		},
		{
			Long:        "--fail-on",
			Type:        "string",
			Default:     "none",
			Description: "Fail if matches at or above severity.",
		},
		{
			Long:        "--baseline",
			Type:        "string",
			Default:     "none",
			Description: "Baseline JSON path for suppression.",
		},
		{
			Long:        "--write-baseline",
			Type:        "bool",
			Default:     "false",
			Description: "Generate/regenerate baseline from findings.",
		},
	}
}

func analyzeHelpGitFlags() []HelpFlag {
	return []HelpFlag{
		{
			Long:        "--git-mode",
			Type:        "string",
			Default:     "off",
			Description: "Select Git mode: off, staged, or diff.",
		},
		{
			Long:        "--git-diff",
			Type:        "string",
			Default:     "none",
			Description: "Diff target/range for --git-mode=diff.",
		},
		{
			Long:        "--git-added-lines-only",
			Type:        "bool",
			Default:     "false",
			Description: "Restrict matches to added lines in Git mode.",
		},
		{
			Long:        "--no-gitignore",
			Type:        "bool",
			Default:     "false",
			Description: "Disable .gitignore filtering for this run.",
		},
	}
}

func analyzeHelpIgnoreFlags() []HelpFlag {
	return []HelpFlag{
		{
			Long:        "--no-ignore-files",
			Type:        "bool",
			Default:     "false",
			Description: "Disable ignore file loading and matching.",
		},
	}
}

func initHelpTopic() HelpTopic {
	return HelpTopic{
		Name:  "init",
		Usage: []string{"reglint init [flags]"},
		Flags: []HelpFlag{
			helpFlag(),
			{
				Long:        "--out",
				Type:        "string",
				Default:     defaultInitPath,
				Description: "Output path for the config file.",
			},
			{
				Long:        "--force",
				Type:        "bool",
				Default:     "false",
				Description: "Overwrite existing config file.",
			},
		},
	}
}

func versionHelpTopic() HelpTopic {
	return HelpTopic{
		Name:  "version",
		Usage: []string{"reglint version"},
		Flags: []HelpFlag{
			helpFlag(),
		},
	}
}

func helpFlag() HelpFlag {
	return HelpFlag{
		Long:        "--help",
		Short:       "-h",
		Type:        "bool",
		Default:     "false",
		Description: "Print help and exit.",
	}
}
