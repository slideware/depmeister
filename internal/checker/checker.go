package checker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/slideware/depmeister/internal/config"
	"github.com/slideware/depmeister/internal/depsdev"
	"github.com/slideware/depmeister/internal/parser"
)

// VersionGetter abstracts the deps.dev API calls for testability.
type VersionGetter interface {
	GetVersion(ctx context.Context, system, name, version string) (*depsdev.VersionResponse, error)
	GetProject(ctx context.Context, projectID string) (*depsdev.ProjectResponse, error)
}

// Flag represents a single issue found for a dependency.
type Flag struct {
	Dependency parser.Dependency
	Reason     string
}

// Result holds the outcome of a check run.
type Result struct {
	Checked int
	Flags   []Flag
}

// Check inspects all dependencies in parallel and returns flagged issues.
func Check(ctx context.Context, cfg config.Config, deps []parser.Dependency, client VersionGetter) (*Result, error) {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		flags   []Flag
		checked int
	)

	sem := make(chan struct{}, max(cfg.Concurrency, 1))

	for _, dep := range deps {
		wg.Add(1)
		go func(dep parser.Dependency) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			f, ok := checkDep(ctx, cfg, dep, client)
			mu.Lock()
			defer mu.Unlock()
			if ok {
				checked++
			}
			flags = append(flags, f...)
		}(dep)
	}

	wg.Wait()

	return &Result{Checked: checked, Flags: flags}, nil
}

// allowlistFor returns the AllowlistEntry for the given dependency name, if any.
func allowlistFor(cfg config.Config, name string) *config.AllowlistEntry {
	for _, e := range cfg.Allowlist {
		if e.Name == name {
			return &e
		}
	}
	return nil
}

// checkDep checks a single dependency. It returns any flags found and whether
// the dependency was successfully checked (API call succeeded).
func checkDep(ctx context.Context, cfg config.Config, dep parser.Dependency, client VersionGetter) ([]Flag, bool) {
	allow := allowlistFor(cfg, dep.Name)

	resp, err := client.GetVersion(ctx, string(dep.System), dep.Name, dep.Version)
	if err != nil {
		slog.Warn("failed to fetch version info, skipping dependency",
			"name", dep.Name,
			"version", dep.Version,
			"error", err,
		)
		return nil, false
	}

	var flags []Flag

	// Age check.
	skipAge := allow != nil && allow.Age
	if !skipAge && cfg.MaxAgeDays > 0 {
		ageDays := time.Since(resp.PublishedAt).Hours() / 24
		if ageDays < float64(cfg.MaxAgeDays) {
			flags = append(flags, Flag{
				Dependency: dep,
				Reason:     fmt.Sprintf("published %.0f days ago (threshold: %d)", ageDays, cfg.MaxAgeDays),
			})
		}
	}

	// Vulnerability check.
	skipVulns := allow != nil && allow.Vulns
	if !skipVulns && cfg.FailOnVulns && len(resp.AdvisoryKeys) > 0 {
		for _, adv := range resp.AdvisoryKeys {
			flags = append(flags, Flag{
				Dependency: dep,
				Reason:     fmt.Sprintf("vulnerability: %s", adv.ID),
			})
		}
	}

	// Scorecard check.
	skipScore := allow != nil && allow.Score
	if !skipScore && cfg.MinScorecard > 0 {
		checkScorecard(ctx, cfg, dep, resp, client, &flags)
	}

	return flags, true
}

func checkScorecard(ctx context.Context, cfg config.Config, dep parser.Dependency, resp *depsdev.VersionResponse, client VersionGetter, flags *[]Flag) {
	// Find a related project that looks like a source repo.
	var projectID string
	for _, rp := range resp.RelatedProjects {
		if rp.RelationType == "ISSUE_TRACKER" || rp.RelationType == "SOURCE_REPO" {
			projectID = rp.ProjectKey.ID
			break
		}
	}
	if projectID == "" {
		slog.Debug("no related project found for scorecard check",
			"name", dep.Name,
		)
		return
	}

	proj, err := client.GetProject(ctx, projectID)
	if err != nil {
		slog.Debug("failed to fetch project info, skipping scorecard check",
			"name", dep.Name,
			"project", projectID,
			"error", err,
		)
		return
	}

	if proj.Scorecard == nil {
		slog.Debug("no scorecard data available",
			"name", dep.Name,
			"project", projectID,
		)
		return
	}

	if proj.Scorecard.OverallScore < cfg.MinScorecard {
		*flags = append(*flags, Flag{
			Dependency: dep,
			Reason:     fmt.Sprintf("scorecard %.1f (threshold: %.1f)", proj.Scorecard.OverallScore, cfg.MinScorecard),
		})
	}
}
