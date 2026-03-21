package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseGoMod reads a go.mod file and returns the declared dependencies.
func ParseGoMod(path string) ([]Dependency, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var deps []Dependency
	inBlock := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Start of a require block.
		if strings.HasPrefix(line, "require (") || line == "require(" {
			inBlock = true
			continue
		}

		// End of a require block.
		if inBlock && line == ")" {
			inBlock = false
			continue
		}

		if inBlock {
			dep, ok := parseRequireLine(line)
			if ok {
				deps = append(deps, dep)
			}
			continue
		}

		// Single-line require statement: require github.com/foo/bar v1.2.3
		if strings.HasPrefix(line, "require ") && !strings.HasPrefix(line, "require (") {
			rest := strings.TrimPrefix(line, "require ")
			dep, ok := parseRequireLine(rest)
			if ok {
				deps = append(deps, dep)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return deps, nil
}

// parseRequireLine parses a single dependency line such as:
//
//	github.com/foo/bar v1.2.3
//	github.com/foo/bar v1.2.3 // indirect
func parseRequireLine(line string) (Dependency, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") {
		return Dependency{}, false
	}

	indirect := strings.Contains(line, "// indirect")

	// Remove inline comments for field parsing.
	if idx := strings.Index(line, "//"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Dependency{}, false
	}

	return Dependency{
		Name:     fields[0],
		Version:  fields[1],
		System:   SystemGo,
		Indirect: indirect,
	}, true
}
