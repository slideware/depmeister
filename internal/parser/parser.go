package parser

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type System string

const (
	SystemGo    System = "GO"
	SystemNPM   System = "NPM"
	SystemPyPI  System = "PYPI"
	SystemCargo System = "CARGO"
)

type Dependency struct {
	Name     string
	Version  string
	System   System
	Source   string // file path where this dependency was found
	Indirect bool   // go.mod // indirect
	Dev      bool   // npm devDependencies
}

// Scan discovers dependency files in dir and parses them.
// If recursive is true, subdirectories are walked as well.
func Scan(dir string, skipIndirect, skipDevDeps, recursive bool) ([]Dependency, error) {
	var dirs []string

	if recursive {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip common dependency/build directories.
				name := d.Name()
				if name == "node_modules" || name == ".git" || name == "vendor" || name == "__pycache__" {
					return filepath.SkipDir
				}
				dirs = append(dirs, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking directory tree: %w", err)
		}
	} else {
		dirs = []string{dir}
	}

	var deps []Dependency
	for _, d := range dirs {
		found, err := scanDir(d)
		if err != nil {
			return nil, err
		}
		deps = append(deps, found...)
	}

	// Filter
	if skipIndirect || skipDevDeps {
		filtered := deps[:0]
		for _, d := range deps {
			if skipIndirect && d.Indirect {
				slog.Debug("skipping indirect dependency", "name", d.Name)
				continue
			}
			if skipDevDeps && d.Dev {
				slog.Debug("skipping dev dependency", "name", d.Name)
				continue
			}
			filtered = append(filtered, d)
		}
		deps = filtered
	}

	return deps, nil
}

func scanDir(dir string) ([]Dependency, error) {
	var deps []Dependency

	// Go
	gomodPath := filepath.Join(dir, "go.mod")
	if fileExists(gomodPath) {
		slog.Debug("found go.mod", "path", gomodPath)
		goDeps, err := ParseGoMod(gomodPath)
		if err != nil {
			return nil, fmt.Errorf("parsing go.mod: %w", err)
		}
		setSource(goDeps, gomodPath)
		deps = append(deps, goDeps...)
	}

	// NPM
	lockPath := filepath.Join(dir, "package-lock.json")
	pkgPath := filepath.Join(dir, "package.json")
	if fileExists(lockPath) {
		slog.Debug("found package-lock.json", "path", lockPath)
		npmDeps, err := ParsePackageLock(lockPath)
		if err != nil {
			return nil, fmt.Errorf("parsing package-lock.json: %w", err)
		}
		setSource(npmDeps, lockPath)
		deps = append(deps, npmDeps...)
	} else if fileExists(pkgPath) {
		slog.Debug("found package.json (no lockfile)", "path", pkgPath)
		npmDeps, err := ParsePackageJSON(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("parsing package.json: %w", err)
		}
		setSource(npmDeps, pkgPath)
		deps = append(deps, npmDeps...)
	}

	// Python
	reqPath := filepath.Join(dir, "requirements.txt")
	if fileExists(reqPath) {
		slog.Debug("found requirements.txt", "path", reqPath)
		pyDeps, err := ParseRequirements(reqPath)
		if err != nil {
			return nil, fmt.Errorf("parsing requirements.txt: %w", err)
		}
		setSource(pyDeps, reqPath)
		deps = append(deps, pyDeps...)
	}

	// Rust
	cargoLockPath := filepath.Join(dir, "Cargo.lock")
	cargoTomlPath := filepath.Join(dir, "Cargo.toml")
	if fileExists(cargoLockPath) {
		slog.Debug("found Cargo.lock", "path", cargoLockPath)
		cargoDeps, err := ParseCargoLock(cargoLockPath)
		if err != nil {
			return nil, fmt.Errorf("parsing Cargo.lock: %w", err)
		}
		setSource(cargoDeps, cargoLockPath)
		deps = append(deps, cargoDeps...)
	} else if fileExists(cargoTomlPath) {
		slog.Debug("found Cargo.toml (no lockfile)", "path", cargoTomlPath)
		cargoDeps, err := ParseCargoToml(cargoTomlPath)
		if err != nil {
			return nil, fmt.Errorf("parsing Cargo.toml: %w", err)
		}
		setSource(cargoDeps, cargoTomlPath)
		deps = append(deps, cargoDeps...)
	}

	return deps, nil
}

func setSource(deps []Dependency, path string) {
	for i := range deps {
		deps[i].Source = path
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
