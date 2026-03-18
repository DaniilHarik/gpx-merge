package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunFailsOnInvalidFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "valid1.gpx"), []byte(testGPX("Track One", 30)), 0o644); err != nil {
		t.Fatalf("write valid1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "valid2.gpx"), []byte(testGPX("Track Two", 25)), 0o644); err != nil {
		t.Fatalf("write valid2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "invalid.gpx"), []byte("<gpx><trk>"), 0o644); err != nil {
		t.Fatalf("write invalid: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "merged.gpx")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"--input", root,
		"--output", outPath,
		"--workers", "4",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() code = %d, want 1\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	// No output should be written when processing fails.
	if _, err := os.Stat(outPath); err == nil {
		t.Fatal("output file should not exist when processing fails")
	}
	// Error must surface to stderr.
	if !strings.Contains(stderr.String(), "process files:") {
		t.Fatalf("stderr missing error: %s", stderr.String())
	}
}

func TestRunDeterministicAcrossWorkerCounts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		path := filepath.Join(root, fmt.Sprintf("track_%d.gpx", i))
		if err := os.WriteFile(path, []byte(testGPX(fmt.Sprintf("Track %d", i), 120)), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	outDir := t.TempDir()
	out1 := filepath.Join(outDir, "merged_w1.gpx")
	out8 := filepath.Join(outDir, "merged_w8.gpx")

	var s1, e1 bytes.Buffer
	code1 := Run(context.Background(), []string{"--input", root, "--output", out1, "--workers", "1"}, &s1, &e1)
	if code1 != 0 {
		t.Fatalf("workers=1 code = %d stderr=%s", code1, e1.String())
	}

	var s8, e8 bytes.Buffer
	code8 := Run(context.Background(), []string{"--input", root, "--output", out8, "--workers", "8"}, &s8, &e8)
	if code8 != 0 {
		t.Fatalf("workers=8 code = %d stderr=%s", code8, e8.String())
	}

	b1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatalf("read out1: %v", err)
	}
	b8, err := os.ReadFile(out8)
	if err != nil {
		t.Fatalf("read out8: %v", err)
	}
	if !bytes.Equal(b1, b8) {
		t.Fatalf("outputs differ across worker counts")
	}
}

func TestRunWarnsOnLargeSegmentDiscontinuity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gap.gpx"), []byte(testGPXWithLargeGap()), 0o644); err != nil {
		t.Fatalf("write gap.gpx: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "merged.gpx")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"--input", root,
		"--output", outPath,
		"--workers", "1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0\nstderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Warnings:\n- gap.gpx: segment discontinuities:") {
		t.Fatalf("warning section missing expected discontinuity warning:\n%s", stdout.String())
	}

	outBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if got := strings.Count(string(outBytes), "<trk>"); got != 2 {
		t.Fatalf("track count = %d, want 2", got)
	}
}

func TestRunSortSegmentsByTimeFixesOutOfOrderSegments(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "out_of_order.gpx"), []byte(testGPXOutOfOrderSegmentsWithTimes()), 0o644); err != nil {
		t.Fatalf("write out_of_order.gpx: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "merged.gpx")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"--input", root,
		"--output", outPath,
		"--workers", "1",
		"--sort-segments-by-time",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0\nstderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Warnings:\n- out_of_order.gpx: reordered segments by first timestamp in 1 track(s) (--sort-segments-by-time)") {
		t.Fatalf("missing segment reorder warning:\n%s", stdout.String())
	}

	outBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if got := strings.Count(string(outBytes), "<trk>"); got != 1 {
		t.Fatalf("track count = %d, want 1", got)
	}
}

func TestRunAppendsMetricsCSV(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "track.gpx"), []byte(testGPX("CSV Metrics", 40)), 0o644); err != nil {
		t.Fatalf("write track.gpx: %v", err)
	}

	metricsPath := filepath.Join(root, "logs", "metrics.csv")

	var stdout1 bytes.Buffer
	var stderr1 bytes.Buffer
	code1 := Run(context.Background(), []string{
		"--input", root,
		"--dry-run",
		"--workers", "2",
		"--metrics-csv", metricsPath,
	}, &stdout1, &stderr1)
	if code1 != 0 {
		t.Fatalf("first run code = %d, want 0\nstderr=%s", code1, stderr1.String())
	}

	var stdout2 bytes.Buffer
	var stderr2 bytes.Buffer
	code2 := Run(context.Background(), []string{
		"--input", root,
		"--dry-run",
		"--workers", "5",
		"--metrics-csv", metricsPath,
	}, &stdout2, &stderr2)
	if code2 != 0 {
		t.Fatalf("second run code = %d, want 0\nstderr=%s", code2, stderr2.String())
	}

	f, err := os.Open(metricsPath)
	if err != nil {
		t.Fatalf("open metrics csv: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read metrics csv: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}

	wantHeader := []string{"started_at_utc", "points_in", "points_out", "workers", "duration_ms", "mb_in", "mb_out"}
	if !equalStringSlices(rows[0], wantHeader) {
		t.Fatalf("header = %v, want %v", rows[0], wantHeader)
	}

	if rows[1][1] != "40" {
		t.Fatalf("first row points_in = %q, want 40", rows[1][1])
	}
	if rows[1][3] != "2" {
		t.Fatalf("first row workers = %q, want 2", rows[1][3])
	}
	if rows[2][1] != "40" {
		t.Fatalf("second row points_in = %q, want 40", rows[2][1])
	}
	if rows[2][3] != "5" {
		t.Fatalf("second row workers = %q, want 5", rows[2][3])
	}

	for i := 1; i <= 2; i++ {
		if _, err := time.Parse(time.RFC3339, rows[i][0]); err != nil {
			t.Fatalf("row %d timestamp parse error: %v", i, err)
		}
		pointsOut, err := strconv.Atoi(rows[i][2])
		if err != nil {
			t.Fatalf("row %d points_out parse error: %v", i, err)
		}
		if pointsOut <= 0 || pointsOut > 40 {
			t.Fatalf("row %d points_out = %d, want 1..40", i, pointsOut)
		}

		durationMs, err := strconv.ParseInt(rows[i][4], 10, 64)
		if err != nil {
			t.Fatalf("row %d duration parse error: %v", i, err)
		}
		if durationMs < 0 {
			t.Fatalf("row %d duration = %d, want >= 0", i, durationMs)
		}

		mbIn, err := strconv.ParseFloat(rows[i][5], 64)
		if err != nil {
			t.Fatalf("row %d mb_in parse error: %v", i, err)
		}
		if mbIn <= 0 {
			t.Fatalf("row %d mb_in = %f, want > 0", i, mbIn)
		}

		mbOut, err := strconv.ParseFloat(rows[i][6], 64)
		if err != nil {
			t.Fatalf("row %d mb_out parse error: %v", i, err)
		}
		if mbOut <= 0 {
			t.Fatalf("row %d mb_out = %f, want > 0", i, mbOut)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func testGPX(name string, points int) string {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	sb.WriteString("<gpx xmlns=\"http://www.topografix.com/GPX/1/1\" version=\"1.1\" creator=\"test\">")
	sb.WriteString("<trk><name>")
	sb.WriteString(name)
	sb.WriteString("</name><trkseg>")
	for i := 0; i < points; i++ {
		lat := 58.0 + float64(i)*0.00001
		lon := 24.0 + float64(i%7)*0.00002
		sb.WriteString(fmt.Sprintf("<trkpt lat=\"%.14f\" lon=\"%.14f\"><ele>%d</ele><time>2024-01-01T00:%02d:00Z</time></trkpt>", lat, lon, 10+i, i%60))
	}
	sb.WriteString("</trkseg></trk></gpx>")
	return sb.String()
}

func testGPXWithLargeGap() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1" creator="test">
  <trk>
    <name>Gap Track</name>
    <trkseg>
      <trkpt lat="58.000000" lon="24.000000"></trkpt>
      <trkpt lat="58.000100" lon="24.000100"></trkpt>
    </trkseg>
    <trkseg>
      <trkpt lat="58.200000" lon="24.200000"></trkpt>
      <trkpt lat="58.200100" lon="24.200100"></trkpt>
    </trkseg>
  </trk>
</gpx>`
}

func testGPXOutOfOrderSegmentsWithTimes() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1" creator="test">
  <trk>
    <name>Out Of Order</name>
    <trkseg>
      <trkpt lat="58.050000" lon="24.050000"><time>2024-06-02T10:00:00Z</time></trkpt>
      <trkpt lat="58.100000" lon="24.100000"><time>2024-06-02T10:10:00Z</time></trkpt>
    </trkseg>
    <trkseg>
      <trkpt lat="58.000000" lon="24.000000"><time>2024-06-01T10:00:00Z</time></trkpt>
      <trkpt lat="58.050100" lon="24.050100"><time>2024-06-01T10:10:00Z</time></trkpt>
    </trkseg>
  </trk>
</gpx>`
}
