package cli

import (
	"path/filepath"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()

	cfg, err := Parse([]string{"--input", temp})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Input != temp {
		t.Fatalf("Input = %q, want %q", cfg.Input, temp)
	}
	if cfg.Output != "./out/merged_optimized.gpx" {
		t.Fatalf("Output = %q", cfg.Output)
	}
	if cfg.Workers != 16 {
		t.Fatalf("Workers = %d", cfg.Workers)
	}
	if cfg.SimplifyMeters != 0.8 {
		t.Fatalf("SimplifyMeters = %v", cfg.SimplifyMeters)
	}
	if cfg.MaxErrorMeters != 1.5 {
		t.Fatalf("MaxErrorMeters = %v", cfg.MaxErrorMeters)
	}
	if cfg.SplitTrackGapMeters != 1000 {
		t.Fatalf("SplitTrackGapMeters = %v", cfg.SplitTrackGapMeters)
	}
	if cfg.SortSegmentsByTime {
		t.Fatalf("SortSegmentsByTime = true, want false")
	}
	if cfg.Precision != 6 {
		t.Fatalf("Precision = %d", cfg.Precision)
	}
	if cfg.MinPoints != 2 {
		t.Fatalf("MinPoints = %d", cfg.MinPoints)
	}
}

func TestParseValidationErrors(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()

	tests := []struct {
		name string
		args []string
	}{
		{name: "workers", args: []string{"--input", temp, "--workers", "0"}},
		{name: "simplify", args: []string{"--input", temp, "--simplify", "0"}},
		{name: "max-error", args: []string{"--input", temp, "--max-error", "0"}},
		{name: "split-track-gap", args: []string{"--input", temp, "--split-track-gap", "-1"}},
		{name: "precision", args: []string{"--input", temp, "--precision", "16"}},
		{name: "min-points", args: []string{"--input", temp, "--min-points", "1"}},
		{name: "bad-output", args: []string{"--input", temp, "--output", ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(tc.args); err == nil {
				t.Fatalf("expected error for args %v", tc.args)
			}
		})
	}
}

func TestParseJSONReportPath(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()
	reportPath := filepath.Join(temp, "reports", "run.json")

	cfg, err := Parse([]string{"--input", temp, "--json-report", reportPath})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.JSONReport != reportPath {
		t.Fatalf("JSONReport = %q, want %q", cfg.JSONReport, reportPath)
	}
}

func TestParseSortSegmentsByTime(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()

	cfg, err := Parse([]string{"--input", temp, "--sort-segments-by-time"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !cfg.SortSegmentsByTime {
		t.Fatalf("SortSegmentsByTime = false, want true")
	}
}
