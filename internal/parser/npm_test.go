package parser

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestParsePackageLock_V2(t *testing.T) {
	content := `{
  "lockfileVersion": 2,
  "packages": {
    "": {
      "name": "myapp",
      "version": "1.0.0"
    },
    "node_modules/lodash": {
      "version": "4.17.21"
    },
    "node_modules/@scope/pkg": {
      "version": "2.0.0",
      "dev": true
    },
    "node_modules/express/node_modules/debug": {
      "version": "3.1.0"
    }
  }
}`

	path := writeTempFile(t, "package-lock.json", content)
	deps, err := ParsePackageLock(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	expected := []Dependency{
		{Name: "@scope/pkg", Version: "2.0.0", System: SystemNPM, Dev: true},
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

func TestParsePackageLock_V3(t *testing.T) {
	content := `{
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "myapp",
      "version": "1.0.0"
    },
    "node_modules/react": {
      "version": "18.2.0"
    }
  }
}`

	path := writeTempFile(t, "package-lock.json", content)
	deps, err := ParsePackageLock(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("got %d deps, want 1", len(deps))
	}
	if deps[0].Name != "react" || deps[0].Version != "18.2.0" || deps[0].System != SystemNPM {
		t.Errorf("got %+v, want react@18.2.0", deps[0])
	}
}

func TestParsePackageLock_V1(t *testing.T) {
	content := `{
  "lockfileVersion": 1,
  "dependencies": {
    "express": {
      "version": "4.18.2"
    },
    "@types/node": {
      "version": "20.4.5",
      "dev": true
    }
  }
}`

	path := writeTempFile(t, "package-lock.json", content)
	deps, err := ParsePackageLock(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	expected := []Dependency{
		{Name: "@types/node", Version: "20.4.5", System: SystemNPM, Dev: true},
		{Name: "express", Version: "4.18.2", System: SystemNPM},
	}

	if len(deps) != len(expected) {
		t.Fatalf("got %d deps, want %d", len(deps), len(expected))
	}
	for i, got := range deps {
		want := expected[i]
		if got != want {
			t.Errorf("dep[%d] = %+v, want %+v", i, got, want)
		}
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

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
