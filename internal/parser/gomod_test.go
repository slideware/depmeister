package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod(t *testing.T) {
	const gomod = `module example.com/myproject

go 1.21

require (
	github.com/stretchr/testify v1.8.4
	golang.org/x/net v0.17.0 // indirect
)

require github.com/single/dep v0.1.0
`

	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte(gomod), 0o644); err != nil {
		t.Fatalf("write temp go.mod: %v", err)
	}

	deps, err := ParseGoMod(path)
	if err != nil {
		t.Fatalf("ParseGoMod returned error: %v", err)
	}

	type want struct {
		name     string
		version  string
		system   System
		indirect bool
	}

	tests := []want{
		{name: "github.com/stretchr/testify", version: "v1.8.4", system: SystemGo, indirect: false},
		{name: "golang.org/x/net", version: "v0.17.0", system: SystemGo, indirect: true},
		{name: "github.com/single/dep", version: "v0.1.0", system: SystemGo, indirect: false},
	}

	if len(deps) != len(tests) {
		t.Fatalf("got %d dependencies, want %d", len(deps), len(tests))
	}

	for i, tt := range tests {
		d := deps[i]
		if d.Name != tt.name {
			t.Errorf("deps[%d].Name = %q, want %q", i, d.Name, tt.name)
		}
		if d.Version != tt.version {
			t.Errorf("deps[%d].Version = %q, want %q", i, d.Version, tt.version)
		}
		if d.System != tt.system {
			t.Errorf("deps[%d].System = %q, want %q", i, d.System, tt.system)
		}
		if d.Indirect != tt.indirect {
			t.Errorf("deps[%d].Indirect = %v, want %v", i, d.Indirect, tt.indirect)
		}
		if d.Dev != false {
			t.Errorf("deps[%d].Dev = %v, want false", i, d.Dev)
		}
	}
}

func TestParseGoMod_FileNotFound(t *testing.T) {
	_, err := ParseGoMod("/nonexistent/go.mod")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseGoMod_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("module example.com/empty\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write temp go.mod: %v", err)
	}

	deps, err := ParseGoMod(path)
	if err != nil {
		t.Fatalf("ParseGoMod returned error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("got %d dependencies, want 0", len(deps))
	}
}
