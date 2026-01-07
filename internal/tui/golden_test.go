package tui

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// stripANSI removes ANSI escape codes from a string for comparison.
// This allows golden tests to focus on content rather than styling.
func stripANSI(s string) string {
	// ANSI escape code pattern
	ansi := regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")
	return ansi.ReplaceAllString(s, "")
}

// normalizeWhitespace normalizes whitespace for golden file comparison.
// - Trims trailing whitespace from each line
// - Collapses multiple consecutive spaces to single spaces (within lines)
// - Preserves newlines
func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		// Trim trailing whitespace
		lines[i] = strings.TrimRight(lines[i], " \t")
		// Collapse multiple spaces (but preserve leading indentation)
		lines[i] = strings.TrimLeft(lines[i], " \t") + " " + strings.TrimRight(lines[i], " \t")
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "\n")
}

// GoldenTest compares rendered output against a golden file.
// If updateGolden is true, writes the golden file instead.
func GoldenTest(t *testing.T, name string, got string) {
	t.Helper()

	// Strip ANSI codes for content-focused comparison
	stripped := stripANSI(got)

	// Golden file path
	goldenPath := filepath.Join("testdata", "golden", name+".golden")

	if *updateGolden {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("failed to create golden directory: %v", err)
		}
		// Write golden file
		if err := os.WriteFile(goldenPath, []byte(stripped), 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	// Read golden file
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v (run with -update-golden to create)", goldenPath, err)
	}

	// Compare
	wantStr := string(want)
	gotStr := stripped

	if wantStr != gotStr {
		// Generate diff
		t.Errorf("golden output mismatch for %s", name)
		diff := diffOutput(wantStr, gotStr)
		t.Logf("\n--- WANT (golden)\n+++ GOT (actual)\n%s", diff)
		t.Logf("To update golden file: go test -update-golden -run %s", t.Name())
	}
}

// diffOutput generates a unified diff-like output for comparing strings.
func diffOutput(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")

	maxLines := len(wantLines)
	if len(gotLines) > maxLines {
		maxLines = len(gotLines)
	}

	var buf bytes.Buffer
	for i := 0; i < maxLines; i++ {
		wantLine := ""
		gotLine := ""
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}

		if wantLine != gotLine {
			if i < len(wantLines) {
				buf.WriteString("--- ")
				buf.WriteString(wantLine)
				buf.WriteString("\n")
			}
			if i < len(gotLines) {
				buf.WriteString("+++ ")
				buf.WriteString(gotLine)
				buf.WriteString("\n")
			}
		}
	}

	return buf.String()
}

// LipglossWidth returns the display width of a string (accounting for ANSI codes).
// This is a wrapper around lipgloss.Width for convenience in tests.
func LipglossWidth(s string) int {
	return lipgloss.Width(s)
}

// StripANSIForExport removes ANSI codes and returns plain text for documentation.
// Useful when generating documentation from TUI components.
func StripANSIForExport(s string) string {
	stripped := stripANSI(s)
	lines := strings.Split(stripped, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}
