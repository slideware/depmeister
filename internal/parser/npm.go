package parser

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const nodeModulesDir = "node_modules/"

// packageLockFile represents the top-level structure of package-lock.json.
type packageLockFile struct {
	LockfileVersion int                           `json:"lockfileVersion"`
	Packages        map[string]packageLockEntry   `json:"packages"`
	Dependencies    map[string]packageLockV1Entry `json:"dependencies"`
}

// packageLockEntry is an entry in the "packages" map of lockfile v2/v3. The
// root entry ("") and workspace entries list the declared dependencies.
type packageLockEntry struct {
	Version              string            `json:"version"`
	Dev                  bool              `json:"dev"`
	Link                 bool              `json:"link"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

// packageLockV1Entry is an entry in the "dependencies" map of lockfile v1 (also
// kept as a legacy section in v2). Its nested "dependencies" holds objects
// rather than version ranges, so it cannot share packageLockEntry.
type packageLockV1Entry struct {
	Version string `json:"version"`
	Dev     bool   `json:"dev"`
}

// packageJSONFile represents the top-level structure of package.json.
type packageJSONFile struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

// ParsePackageLock parses an npm package-lock.json file and returns the
// installed dependencies. It supports lockfile versions 1, 2, and 3.
// Dependencies not declared by the project (or one of its workspaces) are
// marked Indirect. Lockfile v1 does not record the declared dependencies, so
// they are read from package.json in the same directory when it exists.
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
		return parseLockV1(lock.Dependencies, declaredFromPackageJSON(path)), nil
	default:
		return nil, fmt.Errorf("unsupported lockfileVersion %d in %s", lock.LockfileVersion, path)
	}
}

func parseLockV2V3(packages map[string]packageLockEntry) []Dependency {
	// Declared dependency names per install prefix: "" for the project root and
	// "<workspace path>/" for each workspace package.
	declared := make(map[string]map[string]bool)
	for key, entry := range packages {
		if strings.Contains(key, nodeModulesDir) {
			continue
		}
		prefix := key
		if prefix != "" {
			prefix += "/"
		}
		declared[prefix] = declaredNames(entry.Dependencies, entry.DevDependencies, entry.OptionalDependencies, entry.PeerDependencies)
	}

	var deps []Dependency
	for key, entry := range packages {
		// Skip the root, workspace packages and links to them: they are the
		// project's own packages, not dependencies.
		if !strings.Contains(key, nodeModulesDir) || entry.Link {
			continue
		}

		// Skip nested node_modules (transitive vendored copies).
		// These contain "node_modules/" more than once.
		if strings.Count(key, nodeModulesDir) > 1 {
			continue
		}

		prefix, name, _ := strings.Cut(key, nodeModulesDir)
		deps = append(deps, Dependency{
			Name:     name,
			Version:  entry.Version,
			System:   SystemNPM,
			Indirect: !isDeclared(declared, packages, prefix, name),
			Dev:      entry.Dev,
		})
	}
	return deps
}

// isDeclared reports whether the package installed at
// <prefix>node_modules/<name> is declared by the project or workspace that
// resolves to it.
func isDeclared(declared map[string]map[string]bool, packages map[string]packageLockEntry, prefix, name string) bool {
	if declared[prefix][name] {
		return true
	}
	if prefix != "" {
		return false
	}
	// A workspace dependency hoisted to the root is direct, unless the
	// workspace has its own copy and therefore never resolves to this one.
	for ws, names := range declared {
		if ws == "" || !names[name] {
			continue
		}
		if _, ownCopy := packages[ws+nodeModulesDir+name]; !ownCopy {
			return true
		}
	}
	return false
}

// parseLockV1 converts lockfile v1 dependencies. declared holds the names from
// package.json; when nil, all dependencies are reported as direct.
func parseLockV1(dependencies map[string]packageLockV1Entry, declared map[string]bool) []Dependency {
	var deps []Dependency
	for name, entry := range dependencies {
		deps = append(deps, Dependency{
			Name:     name,
			Version:  entry.Version,
			System:   SystemNPM,
			Indirect: declared != nil && !declared[name],
			Dev:      entry.Dev,
		})
	}
	return deps
}

// declaredFromPackageJSON returns the dependency names declared in the
// package.json next to lockPath, or nil if it is missing or unreadable.
func declaredFromPackageJSON(lockPath string) map[string]bool {
	pkgPath := filepath.Join(filepath.Dir(lockPath), "package.json")
	if !fileExists(pkgPath) {
		slog.Debug("no package.json next to lockfile v1, cannot tell direct from indirect dependencies", "path", lockPath)
		return nil
	}

	pkg, err := readPackageJSON(pkgPath)
	if err != nil {
		slog.Warn("cannot read package.json, cannot tell direct from indirect dependencies", "path", pkgPath, "error", err)
		return nil
	}
	return declaredNames(pkg.Dependencies, pkg.DevDependencies, pkg.OptionalDependencies, pkg.PeerDependencies)
}

// declaredNames returns the set of package names present in any of lists.
func declaredNames(lists ...map[string]string) map[string]bool {
	names := make(map[string]bool)
	for _, l := range lists {
		for name := range l {
			names[name] = true
		}
	}
	return names
}

func readPackageJSON(path string) (*packageJSONFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var pkg packageJSONFile
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &pkg, nil
}

// ParsePackageJSON parses an npm package.json file and returns the declared
// dependencies. Because package.json uses semver ranges rather than pinned
// versions, the returned versions are best-effort (^ and ~ prefixes stripped).
func ParsePackageJSON(path string) ([]Dependency, error) {
	pkg, err := readPackageJSON(path)
	if err != nil {
		return nil, err
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
