package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	performanceFixtureFiles  = 64
	performanceFixturePoints = 3000
	defaultPerformanceBudget = 5 * time.Second
)

// TestRunCompletesWithinPerformanceBudget is an end-to-end regression guard for
// the production parse, optimize, and write path. The fixture is created before
// timing starts so filesystem setup is not part of the merge budget.
func TestRunCompletesWithinPerformanceBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance-budget test in short mode")
	}

	budget := performanceBudget(t)
	input := t.TempDir()
	for i := 0; i < performanceFixtureFiles; i++ {
		path := filepath.Join(input, fmt.Sprintf("track_%03d.gpx", i))
		if err := os.WriteFile(path, []byte(testGPX(fmt.Sprintf("Performance %d", i), performanceFixturePoints)), 0o644); err != nil {
			t.Fatalf("write performance fixture %s: %v", path, err)
		}
	}

	output := filepath.Join(t.TempDir(), "merged.gpx")
	var stdout, stderr bytes.Buffer
	started := time.Now()
	code := Run(context.Background(), []string{
		"--input", input,
		"--output", output,
		"--workers", "8",
	}, &stdout, &stderr)
	elapsed := time.Since(started)

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0\nstderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("merged output missing: %v", err)
	}
	if elapsed > budget {
		t.Fatalf("merge took %s, exceeding the %s performance budget; adjust GPX_MERGE_PERFORMANCE_BUDGET only for a deliberately different runner capacity", elapsed.Round(time.Millisecond), budget)
	}
}

func performanceBudget(t *testing.T) time.Duration {
	t.Helper()
	value := os.Getenv("GPX_MERGE_PERFORMANCE_BUDGET")
	if value == "" {
		return defaultPerformanceBudget
	}

	budget, err := time.ParseDuration(value)
	if err != nil || budget <= 0 {
		t.Fatalf("GPX_MERGE_PERFORMANCE_BUDGET must be a positive Go duration, got %q", value)
	}
	return budget
}
