package checker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/slideware/depmeister/internal/config"
	"github.com/slideware/depmeister/internal/depsdev"
	"github.com/slideware/depmeister/internal/parser"
)

// mockClient implements VersionGetter for testing.
type mockClient struct {
	versions map[string]*depsdev.VersionResponse
	projects map[string]*depsdev.ProjectResponse
	errors   map[string]error
}

func (m *mockClient) GetVersion(_ context.Context, system, name, version string) (*depsdev.VersionResponse, error) {
	key := fmt.Sprintf("%s/%s/%s", system, name, version)
	if err, ok := m.errors[key]; ok {
		return nil, err
	}
	if resp, ok := m.versions[key]; ok {
		return resp, nil
	}
	return nil, fmt.Errorf("not found: %s", key)
}

func (m *mockClient) GetProject(_ context.Context, projectID string) (*depsdev.ProjectResponse, error) {
	if err, ok := m.errors[projectID]; ok {
		return nil, err
	}
	if resp, ok := m.projects[projectID]; ok {
		return resp, nil
	}
	return nil, fmt.Errorf("project not found: %s", projectID)
}

func baseCfg() config.Config {
	return config.Config{
		MaxAgeDays:   30,
		MinScorecard: 0,
		FailOnVulns:  true,
		Concurrency:  2,
	}
}

func TestTooNewPackage(t *testing.T) {
	client := &mockClient{
		versions: map[string]*depsdev.VersionResponse{
			"GO/example.com/foo/v1.0.0": {
				PublishedAt: time.Now().Add(-5 * 24 * time.Hour), // 5 days ago
			},
		},
	}

	deps := []parser.Dependency{
		{Name: "example.com/foo", Version: "v1.0.0", System: parser.SystemGo},
	}

	cfg := baseCfg()
	cfg.MaxAgeDays = 30

	res, err := Check(context.Background(), cfg, deps, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Checked != 1 {
		t.Fatalf("expected 1 checked, got %d", res.Checked)
	}
	if len(res.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(res.Flags))
	}
	if res.Flags[0].Dependency.Name != "example.com/foo" {
		t.Errorf("unexpected dependency name: %s", res.Flags[0].Dependency.Name)
	}
	if res.Flags[0].Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

func TestVulnerabilities(t *testing.T) {
	client := &mockClient{
		versions: map[string]*depsdev.VersionResponse{
			"NPM/bad-pkg/1.0.0": {
				PublishedAt: time.Now().Add(-365 * 24 * time.Hour),
				AdvisoryKeys: []depsdev.AdvisoryKey{
					{ID: "GHSA-1234"},
					{ID: "GHSA-5678"},
				},
			},
		},
	}

	deps := []parser.Dependency{
		{Name: "bad-pkg", Version: "1.0.0", System: parser.SystemNPM},
	}

	cfg := baseCfg()
	cfg.FailOnVulns = true

	res, err := Check(context.Background(), cfg, deps, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Checked != 1 {
		t.Fatalf("expected 1 checked, got %d", res.Checked)
	}
	if len(res.Flags) != 2 {
		t.Fatalf("expected 2 flags (one per advisory), got %d", len(res.Flags))
	}
}

func TestLowScorecard(t *testing.T) {
	client := &mockClient{
		versions: map[string]*depsdev.VersionResponse{
			"GO/example.com/low/v0.1.0": {
				PublishedAt: time.Now().Add(-365 * 24 * time.Hour),
				RelatedProjects: []depsdev.RelatedProject{
					{
						ProjectKey: struct {
							ID string `json:"id"`
						}{ID: "github.com/example/low"},
						RelationType: "ISSUE_TRACKER",
					},
				},
			},
		},
		projects: map[string]*depsdev.ProjectResponse{
			"github.com/example/low": {
				Scorecard: &depsdev.Scorecard{
					OverallScore: 3.2,
					Date:         "2026-01-01",
				},
			},
		},
	}

	deps := []parser.Dependency{
		{Name: "example.com/low", Version: "v0.1.0", System: parser.SystemGo},
	}

	cfg := baseCfg()
	cfg.MinScorecard = 7.0

	res, err := Check(context.Background(), cfg, deps, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Checked != 1 {
		t.Fatalf("expected 1 checked, got %d", res.Checked)
	}
	if len(res.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(res.Flags))
	}
	if res.Flags[0].Reason != "scorecard 3.2 (threshold: 7.0)" {
		t.Errorf("unexpected reason: %s", res.Flags[0].Reason)
	}
}

func TestAllowlistSkipsVulns(t *testing.T) {
	client := &mockClient{
		versions: map[string]*depsdev.VersionResponse{
			"NPM/allowed-pkg/2.0.0": {
				PublishedAt: time.Now().Add(-365 * 24 * time.Hour),
				AdvisoryKeys: []depsdev.AdvisoryKey{
					{ID: "GHSA-9999"},
				},
			},
		},
	}

	deps := []parser.Dependency{
		{Name: "allowed-pkg", Version: "2.0.0", System: parser.SystemNPM},
	}

	cfg := baseCfg()
	cfg.FailOnVulns = true
	cfg.Allowlist = []config.AllowlistEntry{
		{Name: "allowed-pkg", Reason: "known safe", Vulns: true},
	}

	res, err := Check(context.Background(), cfg, deps, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Checked != 1 {
		t.Fatalf("expected 1 checked, got %d", res.Checked)
	}
	if len(res.Flags) != 0 {
		t.Fatalf("expected 0 flags (vulns allowlisted), got %d: %+v", len(res.Flags), res.Flags)
	}
}

func TestAPIError(t *testing.T) {
	client := &mockClient{
		errors: map[string]error{
			"GO/broken/v1.0.0": fmt.Errorf("connection refused"),
		},
	}

	deps := []parser.Dependency{
		{Name: "broken", Version: "v1.0.0", System: parser.SystemGo},
	}

	cfg := baseCfg()

	res, err := Check(context.Background(), cfg, deps, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Checked != 0 {
		t.Errorf("expected 0 checked (API error), got %d", res.Checked)
	}
	if len(res.Flags) != 0 {
		t.Errorf("expected 0 flags on API error, got %d", len(res.Flags))
	}
}

func TestNoIssues(t *testing.T) {
	client := &mockClient{
		versions: map[string]*depsdev.VersionResponse{
			"PYPI/good-pkg/3.0.0": {
				PublishedAt: time.Now().Add(-365 * 24 * time.Hour),
			},
		},
	}

	deps := []parser.Dependency{
		{Name: "good-pkg", Version: "3.0.0", System: parser.SystemPyPI},
	}

	cfg := baseCfg()

	res, err := Check(context.Background(), cfg, deps, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Checked != 1 {
		t.Errorf("expected 1 checked, got %d", res.Checked)
	}
	if len(res.Flags) != 0 {
		t.Errorf("expected 0 flags, got %d: %+v", len(res.Flags), res.Flags)
	}
}
