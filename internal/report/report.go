package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/slideware/depmeister/internal/checker"
	"github.com/slideware/depmeister/internal/parser"
)

const (
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiReset = "\033[0m"
)

type jsonReport struct {
	Checked      int              `json:"checked"`
	Issues       []jsonIssue      `json:"issues"`
	Dependencies []jsonDependency `json:"dependencies"`
}

// Dependency statuses used in the JSON report.
const (
	statusOK        = "ok"
	statusFlagged   = "flagged"
	statusUnchecked = "unchecked"
)

type jsonDependency struct {
	Name     string                `json:"name"`
	Version  string                `json:"version"`
	System   string                `json:"system"`
	Source   string                `json:"source"`
	Indirect bool                  `json:"indirect,omitempty"`
	Dev      bool                  `json:"dev,omitempty"`
	Status   string                `json:"status"`
	Issues   []jsonDependencyIssue `json:"issues"`
}

type jsonDependencyIssue struct {
	Reason      string `json:"reason"`
	AdvisoryURL string `json:"advisoryUrl,omitempty"`
}

type jsonIssue struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	System      string `json:"system"`
	Source      string `json:"source"`
	Reason      string `json:"reason"`
	AdvisoryURL string `json:"advisoryUrl,omitempty"`
}

const advisoryURLBase = "https://deps.dev/advisory/osv/"

// Render writes the checker result to w in the given format ("text" or "json").
// When isTTY is true and format is "text", ANSI color codes are used.
func Render(w io.Writer, result *checker.Result, format string, isTTY bool) error {
	switch strings.ToLower(format) {
	case "json":
		return renderJSON(w, result)
	case "text":
		return renderText(w, result, isTTY)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func renderText(w io.Writer, result *checker.Result, isTTY bool) error {
	fmt.Fprintln(w, "Depmeister Report")
	fmt.Fprintf(w, "Checked %d dependencies\n", result.Checked)
	fmt.Fprintln(w)

	if len(result.Flags) == 0 {
		msg := "No issues found."
		if isTTY {
			msg = ansiGreen + msg + ansiReset
		}
		fmt.Fprintln(w, msg)
		return nil
	}

	// Group flags by source file for readability.
	grouped := groupBySource(result.Flags)
	for _, g := range grouped {
		src := g.source
		if src == "" {
			src = "(unknown)"
		}
		fmt.Fprintf(w, "  %s\n", src)
		for _, f := range g.flags {
			tag := "[FLAGGED]"
			if isTTY {
				tag = ansiRed + tag + ansiReset
			}
			fmt.Fprintf(w, "    %s %s@%s: %s\n", tag, f.Dependency.Name, f.Dependency.Version, f.Reason)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%d issue(s) found\n", len(result.Flags))
	return nil
}

func renderJSON(w io.Writer, result *checker.Result) error {
	report := jsonReport{
		Checked:      result.Checked,
		Issues:       make([]jsonIssue, 0, len(result.Flags)),
		Dependencies: make([]jsonDependency, 0, len(result.Dependencies)),
	}

	flagsByDep := make(map[parser.Dependency][]jsonDependencyIssue)
	for _, f := range result.Flags {
		issue := jsonIssue{
			Name:    f.Dependency.Name,
			Version: f.Dependency.Version,
			System:  string(f.Dependency.System),
			Source:  f.Dependency.Source,
			Reason:  f.Reason,
		}
		if f.AdvisoryID != "" {
			issue.AdvisoryURL = advisoryURLBase + f.AdvisoryID
		}
		report.Issues = append(report.Issues, issue)
		flagsByDep[f.Dependency] = append(flagsByDep[f.Dependency], jsonDependencyIssue{
			Reason:      issue.Reason,
			AdvisoryURL: issue.AdvisoryURL,
		})
	}

	for _, d := range result.Dependencies {
		issues := flagsByDep[d.Dependency]
		status := statusOK
		switch {
		case !d.Checked:
			status = statusUnchecked
		case len(issues) > 0:
			status = statusFlagged
		}
		if issues == nil {
			issues = []jsonDependencyIssue{}
		}
		report.Dependencies = append(report.Dependencies, jsonDependency{
			Name:     d.Dependency.Name,
			Version:  d.Dependency.Version,
			System:   string(d.Dependency.System),
			Source:   d.Dependency.Source,
			Indirect: d.Dependency.Indirect,
			Dev:      d.Dependency.Dev,
			Status:   status,
			Issues:   issues,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

type sourceGroup struct {
	source string
	flags  []checker.Flag
}

// groupBySource groups flags by their source file, preserving order of first appearance.
func groupBySource(flags []checker.Flag) []sourceGroup {
	order := make(map[string]int)
	var groups []sourceGroup

	for _, f := range flags {
		src := f.Dependency.Source
		idx, exists := order[src]
		if !exists {
			idx = len(groups)
			order[src] = idx
			groups = append(groups, sourceGroup{source: src})
		}
		groups[idx].flags = append(groups[idx].flags, f)
	}

	return groups
}
