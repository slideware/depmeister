package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRequirements_PinnedPackages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.txt")
	content := "requests==2.31.0\nurllib3==2.0.4\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParseRequirements(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}

	want := []struct {
		name    string
		version string
	}{
		{"requests", "2.31.0"},
		{"urllib3", "2.0.4"},
	}
	for i, w := range want {
		if deps[i].Name != w.name {
			t.Errorf("dep[%d].Name = %q, want %q", i, deps[i].Name, w.name)
		}
		if deps[i].Version != w.version {
			t.Errorf("dep[%d].Version = %q, want %q", i, deps[i].Version, w.version)
		}
		if deps[i].System != SystemPyPI {
			t.Errorf("dep[%d].System = %q, want %q", i, deps[i].System, SystemPyPI)
		}
		if deps[i].Indirect {
			t.Errorf("dep[%d].Indirect = true, want false", i)
		}
		if deps[i].Dev {
			t.Errorf("dep[%d].Dev = true, want false", i)
		}
	}
}

func TestParseRequirements_CommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.txt")
	content := `# This is a comment
requests==2.31.0

# Another comment

urllib3==2.0.4
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParseRequirements(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if deps[0].Name != "requests" {
		t.Errorf("dep[0].Name = %q, want %q", deps[0].Name, "requests")
	}
	if deps[1].Name != "urllib3" {
		t.Errorf("dep[1].Name = %q, want %q", deps[1].Name, "urllib3")
	}
}

func TestParseRequirements_RangesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.txt")
	content := `flask>=2.0
django<=4.0
numpy!=1.21
scipy~=1.7
pandas>1.3
requests==2.31.0
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParseRequirements(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].Name != "requests" {
		t.Errorf("dep[0].Name = %q, want %q", deps[0].Name, "requests")
	}
}

func TestParseRequirements_RecursiveIncludes(t *testing.T) {
	dir := t.TempDir()

	basePath := filepath.Join(dir, "base.txt")
	baseContent := "urllib3==2.0.4\n"
	if err := os.WriteFile(basePath, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	mainPath := filepath.Join(dir, "requirements.txt")
	mainContent := "requests==2.31.0\n-r base.txt\n"
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParseRequirements(mainPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if deps[0].Name != "requests" || deps[0].Version != "2.31.0" {
		t.Errorf("dep[0] = %+v, want requests==2.31.0", deps[0])
	}
	if deps[1].Name != "urllib3" || deps[1].Version != "2.0.4" {
		t.Errorf("dep[1] = %+v, want urllib3==2.0.4", deps[1])
	}
}

func TestParseRequirements_ExtrasInName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.txt")
	content := "requests[security]==2.31.0\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParseRequirements(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].Name != "requests[security]" {
		t.Errorf("dep[0].Name = %q, want %q", deps[0].Name, "requests[security]")
	}
	if deps[0].Version != "2.31.0" {
		t.Errorf("dep[0].Version = %q, want %q", deps[0].Version, "2.31.0")
	}
}
