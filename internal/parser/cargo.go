package parser

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// ParseCargoLock parses a Cargo.lock file and returns all dependencies.
// Cargo.lock uses a TOML-like format with [[package]] blocks.
func ParseCargoLock(path string) ([]Dependency, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []Dependency
	var name, version string
	inPackage := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[[package]]" {
			if inPackage && name != "" && version != "" {
				deps = appendCargoDep(deps, name, version)
			}
			name = ""
			version = ""
			inPackage = true
			continue
		}

		if !inPackage {
			continue
		}

		if k, v, ok := parseTomlKV(line); ok {
			switch k {
			case "name":
				name = v
			case "version":
				version = v
			}
		}
	}

	// Don't forget the last block.
	if inPackage && name != "" && version != "" {
		deps = appendCargoDep(deps, name, version)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading Cargo.lock: %w", err)
	}

	return deps, nil
}

// ParseCargoToml parses a Cargo.toml for dependencies. Only exact versions
// are extracted; version ranges are skipped with a warning.
func ParseCargoToml(path string) ([]Dependency, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []Dependency
	inDeps := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Track section headers.
		if strings.HasPrefix(line, "[") {
			section := strings.Trim(line, "[] ")
			inDeps = section == "dependencies" || section == "dev-dependencies"
			continue
		}

		if !inDeps || line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		k, v, ok := parseTomlKV(line)
		if !ok {
			continue
		}

		// Value can be a simple "version" or a table like { version = "1.0", features = [...] }.
		version := extractCargoVersion(v)
		if version == "" {
			continue
		}

		// Skip version ranges.
		if strings.ContainsAny(version, "^~*><= ") {
			slog.Debug("skipping version range in Cargo.toml", "name", k, "version", version)
			continue
		}

		deps = append(deps, Dependency{
			Name:    k,
			Version: version,
			System:  SystemCargo,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading Cargo.toml: %w", err)
	}

	return deps, nil
}

func appendCargoDep(deps []Dependency, name, version string) []Dependency {
	return append(deps, Dependency{
		Name:    name,
		Version: version,
		System:  SystemCargo,
	})
}

// parseTomlKV parses a line like `key = "value"` and returns (key, unquoted value, true).
func parseTomlKV(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	k := strings.TrimSpace(parts[0])
	v := strings.TrimSpace(parts[1])
	v = strings.Trim(v, "\"")
	return k, v, true
}

// extractCargoVersion handles both simple string values and inline table values
// like { version = "1.0", features = ["derive"] }.
func extractCargoVersion(v string) string {
	if !strings.HasPrefix(v, "{") {
		return v
	}
	// Inline table: find version key.
	inner := strings.Trim(v, "{}")
	for _, part := range strings.Split(inner, ",") {
		k, val, ok := parseTomlKV(strings.TrimSpace(part))
		if ok && k == "version" {
			return val
		}
	}
	return ""
}
