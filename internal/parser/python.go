package parser

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ParseRequirements parses a Python requirements.txt file and returns
// all dependencies that are pinned with ==.
func ParseRequirements(path string) ([]Dependency, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var deps []Dependency
	dir := filepath.Dir(path)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle -r includes (recursive requirements files).
		if refPath, ok := strings.CutPrefix(line, "-r "); ok {
			refPath = strings.TrimSpace(refPath)
			resolved := refPath
			if !filepath.IsAbs(refPath) {
				resolved = filepath.Join(dir, refPath)
			}
			included, err := ParseRequirements(resolved)
			if err != nil {
				return nil, fmt.Errorf("parsing included file %s: %w", resolved, err)
			}
			deps = append(deps, included...)
			continue
		}

		// Skip other pip flags (-e, -i, etc.).
		if strings.HasPrefix(line, "-") {
			continue
		}

		// Check for pinned version with ==.
		if strings.Contains(line, "==") {
			parts := strings.SplitN(line, "==", 2)
			name := strings.TrimSpace(parts[0])
			version := strings.TrimSpace(parts[1])
			deps = append(deps, Dependency{
				Name:    name,
				Version: version,
				System:  SystemPyPI,
			})
			continue
		}

		// Lines with other version operators are skipped.
		for _, op := range []string{">=", "<=", "!=", "~=", ">"} {
			if strings.Contains(line, op) {
				slog.Debug("skipping requirement with unsupported operator", "line", line, "operator", op)
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return deps, nil
}
