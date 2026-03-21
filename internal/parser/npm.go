package parser

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// packageLockFile represents the top-level structure of package-lock.json.
type packageLockFile struct {
	LockfileVersion int                         `json:"lockfileVersion"`
	Packages        map[string]packageLockEntry `json:"packages"`
	Dependencies    map[string]packageLockEntry `json:"dependencies"`
}

type packageLockEntry struct {
	Version string `json:"version"`
	Dev     bool   `json:"dev"`
}

// packageJSONFile represents the top-level structure of package.json.
type packageJSONFile struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// ParsePackageLock parses an npm package-lock.json file and returns the
// declared dependencies. It supports lockfile versions 1, 2, and 3.
func ParsePackageLock(path string) ([]Dependency, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var lock packageLockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	switch lock.LockfileVersion {
	case 2, 3:
		return parseLockV2V3(lock.Packages), nil
	case 1:
		return parseLockV1(lock.Dependencies), nil
	default:
		return nil, fmt.Errorf("unsupported lockfileVersion %d in %s", lock.LockfileVersion, path)
	}
}

func parseLockV2V3(packages map[string]packageLockEntry) []Dependency {
	var deps []Dependency
	for key, entry := range packages {
		// Skip the root entry.
		if key == "" {
			continue
		}

		// Skip nested node_modules (transitive vendored copies).
		// These contain "node_modules/" more than once.
		if strings.Count(key, "node_modules/") > 1 {
			continue
		}

		name := strings.TrimPrefix(key, "node_modules/")
		deps = append(deps, Dependency{
			Name:    name,
			Version: entry.Version,
			System:  SystemNPM,
			Dev:     entry.Dev,
		})
	}
	return deps
}

func parseLockV1(dependencies map[string]packageLockEntry) []Dependency {
	var deps []Dependency
	for name, entry := range dependencies {
		deps = append(deps, Dependency{
			Name:    name,
			Version: entry.Version,
			System:  SystemNPM,
			Dev:     entry.Dev,
		})
	}
	return deps
}

// ParsePackageJSON parses an npm package.json file and returns the declared
// dependencies. Because package.json uses semver ranges rather than pinned
// versions, the returned versions are best-effort (^ and ~ prefixes stripped).
func ParsePackageJSON(path string) ([]Dependency, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var pkg packageJSONFile
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	slog.Warn("package.json versions may be ranges; pinned versions require a lockfile", "path", path)

	var deps []Dependency
	for name, version := range pkg.Dependencies {
		deps = append(deps, Dependency{
			Name:    name,
			Version: stripVersionPrefix(version),
			System:  SystemNPM,
		})
	}
	for name, version := range pkg.DevDependencies {
		deps = append(deps, Dependency{
			Name:    name,
			Version: stripVersionPrefix(version),
			System:  SystemNPM,
			Dev:     true,
		})
	}
	return deps, nil
}

// stripVersionPrefix removes common semver range prefixes (^ and ~).
func stripVersionPrefix(v string) string {
	v = strings.TrimPrefix(v, "^")
	v = strings.TrimPrefix(v, "~")
	return v
}
