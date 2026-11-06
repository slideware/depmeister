package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// lockV2V3Fixture returns a package-lock.json with the given lockfileVersion.
// legacy is spliced in as the top-level "dependencies" section (lockfile v2
// keeps a v1-style copy there for older npm versions).
func lockV2V3Fixture(version int, legacy string) string {
	return fmt.Sprintf(`{
  "lockfileVersion": %d,
  "packages": {
    "": {
      "name": "myapp",
      "version": "1.0.0",
      "dependencies": {"lodash": "^4.17.0", "@scope/pkg": "^2.0.0"},
      "devDependencies": {"jest": "^29.0.0"},
      "optionalDependencies": {"fsevents": "^2.3.0"},
      "peerDependencies": {"react": "^18.0.0"}
    },
    "node_modules/lodash": {"version": "4.17.21"},
    "node_modules/@scope/pkg": {"version": "2.0.0", "dependencies": {"ms": "^2.1.0"}},
    "node_modules/@scope/pkg/node_modules/ms": {"version": "2.0.0"},
    "node_modules/ms": {"version": "2.1.3"},
    "node_modules/jest": {"version": "29.7.0", "dev": true},
    "node_modules/@babel/core": {"version": "7.24.0", "dev": true},
    "node_modules/fsevents": {"version": "2.3.3", "optional": true},
    "node_modules/react": {"version": "18.2.0", "peer": true}
  }%s
}`, version, legacy)
}

func TestParsePackageLock_V2V3(t *testing.T) {
	legacyV2 := `,
  "dependencies": {
    "lodash": {"version": "4.17.21"},
    "@scope/pkg": {
      "version": "2.0.0",
      "requires": {"ms": "^2.1.0"},
      "dependencies": {"ms": {"version": "2.0.0"}}
    }
  }`

	expected := []Dependency{
		{Name: "@babel/core", Version: "7.24.0", System: SystemNPM, Indirect: true, Dev: true},
		{Name: "@scope/pkg", Version: "2.0.0", System: SystemNPM},
		{Name: "fsevents", Version: "2.3.3", System: SystemNPM},
		{Name: "jest", Version: "29.7.0", System: SystemNPM, Dev: true},
		{Name: "lodash", Version: "4.17.21", System: SystemNPM},
		{Name: "ms", Version: "2.1.3", System: SystemNPM, Indirect: true},
		{Name: "react", Version: "18.2.0", System: SystemNPM},
	}

	tests := []struct {
		name    string
		content string
	}{
		{name: "v2", content: lockV2V3Fixture(2, legacyV2)},
		{name: "v3", content: lockV2V3Fixture(3, "")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempFile(t, "package-lock.json", tt.content)
			deps, err := ParsePackageLock(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertDeps(t, deps, expected)
		})
	}
}

func TestParsePackageLock_Workspaces(t *testing.T) {
	content := `{
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "mono",
      "workspaces": ["packages/*"],
      "dependencies": {"lodash": "^4.17.0"}
    },
    "node_modules/lodash": {"version": "4.17.21"},
    "node_modules/foo": {"resolved": "packages/foo", "link": true},
    "packages/foo": {
      "name": "foo",
      "version": "1.0.0",
      "dependencies": {"chalk": "^5.0.0", "ms": "^2.1.0"},
      "devDependencies": {"@types/ms": "^0.7.0"}
    },
    "node_modules/chalk": {"version": "5.3.0"},
    "node_modules/ms": {"version": "2.0.0"},
    "packages/foo/node_modules/ms": {"version": "2.1.3"},
    "packages/foo/node_modules/@types/ms": {"version": "0.7.34", "dev": true},
    "packages/foo/node_modules/supports-color": {"version": "9.4.0"},
    "packages/foo/node_modules/a/node_modules/b": {"version": "1.0.0"}
  }
}`

	path := writeTempFile(t, "package-lock.json", content)
	deps, err := ParsePackageLock(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertDeps(t, deps, []Dependency{
		// Declared by the workspace and installed in its own node_modules.
		{Name: "@types/ms", Version: "0.7.34", System: SystemNPM, Dev: true},
		// Declared by the workspace and hoisted to the root.
		{Name: "chalk", Version: "5.3.0", System: SystemNPM},
		// Declared by the root.
		{Name: "lodash", Version: "4.17.21", System: SystemNPM},
		// The workspace has its own ms, so the hoisted copy belongs to someone else.
		{Name: "ms", Version: "2.0.0", System: SystemNPM, Indirect: true},
		{Name: "ms", Version: "2.1.3", System: SystemNPM},
		{Name: "supports-color", Version: "9.4.0", System: SystemNPM, Indirect: true},
	})
}

func TestParsePackageLock_V1(t *testing.T) {
	lock := `{
  "lockfileVersion": 1,
  "dependencies": {
    "express": {
      "version": "4.18.2",
      "requires": {"debug": "2.6.9"},
      "dependencies": {"ms": {"version": "2.0.0"}}
    },
    "debug": {"version": "2.6.9"},
    "@types/node": {"version": "20.4.5", "dev": true},
    "@types/express": {"version": "4.17.17", "dev": true}
  }
}`
	pkgJSON := `{
  "name": "myapp",
  "dependencies": {"express": "^4.18.0"},
  "devDependencies": {"@types/node": "^20.0.0"}
}`

	tests := []struct {
		name     string
		files    map[string]string
		expected []Dependency
	}{
		{
			name:  "with package.json",
			files: map[string]string{"package-lock.json": lock, "package.json": pkgJSON},
			expected: []Dependency{
				{Name: "@types/express", Version: "4.17.17", System: SystemNPM, Indirect: true, Dev: true},
				{Name: "@types/node", Version: "20.4.5", System: SystemNPM, Dev: true},
				{Name: "debug", Version: "2.6.9", System: SystemNPM, Indirect: true},
				{Name: "express", Version: "4.18.2", System: SystemNPM},
			},
		},
		{
			// Without package.json, direct and indirect cannot be told apart.
			name:  "without package.json",
			files: map[string]string{"package-lock.json": lock},
			expected: []Dependency{
				{Name: "@types/express", Version: "4.17.17", System: SystemNPM, Dev: true},
				{Name: "@types/node", Version: "20.4.5", System: SystemNPM, Dev: true},
				{Name: "debug", Version: "2.6.9", System: SystemNPM},
				{Name: "express", Version: "4.18.2", System: SystemNPM},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTempFiles(t, tt.files)
			deps, err := ParsePackageLock(filepath.Join(dir, "package-lock.json"))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertDeps(t, deps, tt.expected)
		})
	}
}

func TestScan_SkipIndirectNPM(t *testing.T) {
	dir := writeTempFiles(t, map[string]string{
		"package-lock.json": `{
  "lockfileVersion": 3,
  "packages": {
    "": {"name": "myapp", "dependencies": {"lodash": "^4.17.0"}},
    "node_modules/lodash": {"version": "4.17.21"},
    "node_modules/ms": {"version": "2.1.3"}
  }
}`,
	})
	lockPath := filepath.Join(dir, "package-lock.json")

	tests := []struct {
		name         string
		skipIndirect bool
		expected     []Dependency
	}{
		{
			name:         "skip indirect",
			skipIndirect: true,
			expected: []Dependency{
				{Name: "lodash", Version: "4.17.21", System: SystemNPM, Source: lockPath},
			},
		},
		{
			name:         "keep indirect",
			skipIndirect: false,
			expected: []Dependency{
				{Name: "lodash", Version: "4.17.21", System: SystemNPM, Source: lockPath},
				{Name: "ms", Version: "2.1.3", System: SystemNPM, Source: lockPath, Indirect: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, err := Scan(dir, tt.skipIndirect, false, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertDeps(t, deps, tt.expected)
		})
	}
}

func TestParsePackageJSON(t *testing.T) {
	content := `{
  "name": "myapp",
  "dependencies": {
    "lodash": "^4.17.21",
    "@scope/utils": "~1.2.3"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`

	path := writeTempFile(t, "package.json", content)
	deps, err := ParsePackageJSON(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	expected := []Dependency{
		{Name: "@scope/utils", Version: "1.2.3", System: SystemNPM},
		{Name: "jest", Version: "29.0.0", System: SystemNPM, Dev: true},
		{Name: "lodash", Version: "4.17.21", System: SystemNPM},
	}

	if len(deps) != len(expected) {
		t.Fatalf("got %d deps, want %d: %+v", len(deps), len(expected), deps)
	}
	for i, got := range deps {
		want := expected[i]
		if got != want {
			t.Errorf("dep[%d] = %+v, want %+v", i, got, want)
		}
	}
}

// assertDeps compares deps against expected, ignoring order.
func assertDeps(t *testing.T, deps, expected []Dependency) {
	t.Helper()
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Name != deps[j].Name {
			return deps[i].Name < deps[j].Name
		}
		return deps[i].Version < deps[j].Version
	})

	if len(deps) != len(expected) {
		t.Fatalf("got %d deps, want %d: %+v", len(deps), len(expected), deps)
	}
	for i, got := range deps {
		if got != expected[i] {
			t.Errorf("dep[%d] = %+v, want %+v", i, got, expected[i])
		}
	}
}

// writeTempFiles writes files (name to content) into a new temp directory and
// returns its path.
func writeTempFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing temp file: %v", err)
		}
	}
	return dir
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
