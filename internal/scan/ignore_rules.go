package scan

import (
	"path/filepath"

	"github.com/reglint/reglint/internal/ignore"
)

func loadIgnoreRules(request Request) (map[string][]ignore.IgnoreRule, error) {
	if !request.Ignore.Enabled {
		return nil, nil
	}

	byRoot := make(map[string][]ignore.IgnoreRule, len(request.Roots))
	for _, root := range request.Roots {
		root = filepath.Clean(root)
		rules, err := ignore.Load(root, request.Ignore.Files)
		if err != nil {
			return nil, err
		}
		byRoot[root] = rules
	}

	return byRoot, nil
}
