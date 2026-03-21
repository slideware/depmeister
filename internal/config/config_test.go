package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.MaxAgeDays != 7 {
		t.Errorf("MaxAgeDays = %d, want 7", cfg.MaxAgeDays)
	}
	if cfg.FailOnVulns != true {
		t.Error("FailOnVulns should default to true")
	}
	if cfg.SkipIndirect != true {
		t.Error("SkipIndirect should default to true")
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want 10", cfg.Concurrency)
	}
	if cfg.Dir != "." {
		t.Errorf("Dir = %q, want \".\"", cfg.Dir)
	}
	if cfg.Format != "text" {
		t.Errorf("Format = %q, want \"text\"", cfg.Format)
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, ".depmeister.yaml")

	content := []byte(`
max_age_days: 60
min_scorecard: 7.5
fail_on_vulns: false
concurrency: 5
allowlist:
  - name: "my-pkg"
    reason: "internal"
    vulns: true
    age: true
`)
	if err := os.WriteFile(yamlPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.ConfigPath = yamlPath

	cfg, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MaxAgeDays != 60 {
		t.Errorf("MaxAgeDays = %d, want 60", cfg.MaxAgeDays)
	}
	if cfg.MinScorecard != 7.5 {
		t.Errorf("MinScorecard = %f, want 7.5", cfg.MinScorecard)
	}
	if cfg.FailOnVulns != false {
		t.Error("FailOnVulns should be false from YAML")
	}
	if cfg.Concurrency != 5 {
		t.Errorf("Concurrency = %d, want 5", cfg.Concurrency)
	}
	if len(cfg.Allowlist) != 1 {
		t.Fatalf("Allowlist len = %d, want 1", len(cfg.Allowlist))
	}
	if cfg.Allowlist[0].Name != "my-pkg" {
		t.Errorf("Allowlist[0].Name = %q, want \"my-pkg\"", cfg.Allowlist[0].Name)
	}
}

func TestLoadMissingYAML(t *testing.T) {
	cfg := Default()
	cfg.ConfigPath = "/nonexistent/.depmeister.yaml"

	cfg, err := Load(cfg)
	if err != nil {
		t.Fatalf("missing YAML should not error, got: %v", err)
	}
	if cfg.MaxAgeDays != 7 {
		t.Errorf("MaxAgeDays = %d, want default 30", cfg.MaxAgeDays)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("DEPMEISTER_MAX_AGE_DAYS", "90")
	t.Setenv("DEPMEISTER_MIN_SCORECARD", "5.0")
	t.Setenv("DEPMEISTER_FAIL_ON_VULNS", "false")
	t.Setenv("DEPMEISTER_CONCURRENCY", "20")
	t.Setenv("DEPMEISTER_DIR", "/tmp/scan")
	t.Setenv("DEPMEISTER_FORMAT", "JSON")
	t.Setenv("DEPMEISTER_VERBOSE", "1")

	cfg := Default()
	cfg.ConfigPath = "/nonexistent"

	cfg, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MaxAgeDays != 90 {
		t.Errorf("MaxAgeDays = %d, want 90", cfg.MaxAgeDays)
	}
	if cfg.MinScorecard != 5.0 {
		t.Errorf("MinScorecard = %f, want 5.0", cfg.MinScorecard)
	}
	if cfg.FailOnVulns != false {
		t.Error("FailOnVulns should be false from env")
	}
	if cfg.Concurrency != 20 {
		t.Errorf("Concurrency = %d, want 20", cfg.Concurrency)
	}
	if cfg.Dir != "/tmp/scan" {
		t.Errorf("Dir = %q, want \"/tmp/scan\"", cfg.Dir)
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want \"json\"", cfg.Format)
	}
	if cfg.Verbose != true {
		t.Error("Verbose should be true")
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, ".depmeister.yaml")

	content := []byte("max_age_days: 60\n")
	if err := os.WriteFile(yamlPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DEPMEISTER_MAX_AGE_DAYS", "120")

	cfg := Default()
	cfg.ConfigPath = yamlPath

	cfg, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MaxAgeDays != 120 {
		t.Errorf("MaxAgeDays = %d, want 120 (env should override YAML)", cfg.MaxAgeDays)
	}
}

func TestConcurrencyFloor(t *testing.T) {
	t.Setenv("DEPMEISTER_CONCURRENCY", "0")

	cfg := Default()
	cfg.ConfigPath = "/nonexistent"

	cfg, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1 (floor)", cfg.Concurrency)
	}
}
