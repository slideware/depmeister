package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/slideware/depmeister/internal/checker"
	"github.com/slideware/depmeister/internal/parser"
)

func TestTextNoFlags(t *testing.T) {
	var buf bytes.Buffer
	result := &checker.Result{Checked: 5, Flags: nil}

	if err := Render(&buf, result, "text", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No issues found") {
		t.Errorf("expected 'No issues found' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Checked 5 dependencies") {
		t.Errorf("expected 'Checked 5 dependencies' in output, got:\n%s", out)
	}
}

func TestTextWithFlags(t *testing.T) {
	var buf bytes.Buffer
	result := &checker.Result{
		Checked: 3,
		Flags: []checker.Flag{
			{
				Dependency: parser.Dependency{
					Name:    "bad-pkg",
					Version: "1.0.0",
					System:  parser.SystemNPM,
				},
				Reason: "vulnerability: GHSA-1234",
			},
		},
	}

	if err := Render(&buf, result, "text", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[FLAGGED]") {
		t.Errorf("expected '[FLAGGED]' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "bad-pkg") {
		t.Errorf("expected 'bad-pkg' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1 issue(s) found") {
		t.Errorf("expected '1 issue(s) found' in output, got:\n%s", out)
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	result := &checker.Result{
		Checked: 2,
		Flags: []checker.Flag{
			{
				Dependency: parser.Dependency{
					Name:    "example.com/foo",
					Version: "v1.2.0",
					System:  parser.SystemGo,
				},
				Reason: "scorecard 3.2 (threshold: 7.0)",
			},
		},
	}

	if err := Render(&buf, result, "json", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report jsonReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw:\n%s", err, buf.String())
	}

	if report.Checked != 2 {
		t.Errorf("expected checked=2, got %d", report.Checked)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(report.Issues))
	}
	if report.Issues[0].Name != "example.com/foo" {
		t.Errorf("expected name 'example.com/foo', got %q", report.Issues[0].Name)
	}
	if report.Issues[0].Version != "v1.2.0" {
		t.Errorf("expected version 'v1.2.0', got %q", report.Issues[0].Version)
	}
	if report.Issues[0].System != "GO" {
		t.Errorf("expected system 'GO', got %q", report.Issues[0].System)
	}
	if report.Issues[0].Reason != "scorecard 3.2 (threshold: 7.0)" {
		t.Errorf("unexpected reason: %q", report.Issues[0].Reason)
	}
}

func TestTextTTYColors(t *testing.T) {
	var buf bytes.Buffer
	result := &checker.Result{
		Checked: 1,
		Flags: []checker.Flag{
			{
				Dependency: parser.Dependency{
					Name:    "pkg",
					Version: "0.1.0",
					System:  parser.SystemPyPI,
				},
				Reason: "too new",
			},
		},
	}

	if err := Render(&buf, result, "text", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "\033[") {
		t.Errorf("expected ANSI escape codes in TTY output, got:\n%s", out)
	}
	if !strings.Contains(out, ansiRed) {
		t.Errorf("expected red ANSI code for [FLAGGED], got:\n%s", out)
	}
}

func TestTextNoTTYNoColors(t *testing.T) {
	var buf bytes.Buffer
	result := &checker.Result{
		Checked: 1,
		Flags: []checker.Flag{
			{
				Dependency: parser.Dependency{
					Name:    "pkg",
					Version: "0.1.0",
					System:  parser.SystemPyPI,
				},
				Reason: "too new",
			},
		},
	}

	if err := Render(&buf, result, "text", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("expected no ANSI escape codes in non-TTY output, got:\n%s", out)
	}
}
