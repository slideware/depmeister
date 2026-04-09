package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/slideware/depmeister/internal/checker"
	"github.com/slideware/depmeister/internal/config"
	"github.com/slideware/depmeister/internal/depsdev"
	"github.com/slideware/depmeister/internal/parser"
	"github.com/slideware/depmeister/internal/report"

	"golang.org/x/term"
)

func main() {
	cfg := config.Default()

	flag.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "Path to YAML config")
	flag.StringVar(&cfg.Format, "format", cfg.Format, "Output format: text|json")
	flag.StringVar(&cfg.Dir, "dir", cfg.Dir, "Directory to scan")
	flag.BoolVar(&cfg.Recursive, "recursive", cfg.Recursive, "Recursively scan subdirectories")
	flag.BoolVar(&cfg.Verbose, "verbose", cfg.Verbose, "Debug-level logging")
	flag.Parse()

	level := slog.LevelInfo
	if cfg.Verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	cfg, err := config.Load(cfg)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	slog.Debug("config loaded",
		"max_age_days", cfg.MaxAgeDays,
		"min_scorecard", cfg.MinScorecard,
		"fail_on_vulns", cfg.FailOnVulns,
		"concurrency", cfg.Concurrency,
		"dir", cfg.Dir,
		"format", cfg.Format,
	)

	// Scan for dependencies.
	deps, err := parser.Scan(cfg.Dir, cfg.SkipIndirect, cfg.SkipDevDeps, cfg.Recursive)
	if err != nil {
		slog.Error("scanning dependencies", "error", err)
		os.Exit(1)
	}

	if len(deps) == 0 {
		slog.Info("no dependencies found", "dir", cfg.Dir)
		os.Exit(0)
	}

	slog.Debug("dependencies found", "count", len(deps))

	// Check dependencies against deps.dev.
	client := depsdev.NewClient(cfg.Concurrency, "")
	result, err := checker.Check(context.Background(), cfg, deps, client)
	if err != nil {
		slog.Error("checking dependencies", "error", err)
		os.Exit(1)
	}

	// Render report.
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if err := report.Render(os.Stdout, result, cfg.Format, isTTY); err != nil {
		slog.Error("rendering report", "error", err)
		os.Exit(1)
	}

	if len(result.Flags) > 0 {
		os.Exit(1)
	}
}
