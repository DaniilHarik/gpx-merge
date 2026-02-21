package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMixedValidInvalidFiles(t *testing.T) {
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

	outPath := filepath.Join(root, "out", "merged.gpx")
	jsonPath := filepath.Join(root, "report", "run.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"--input", root,
		"--output", outPath,
		"--workers", "4",
		"--json-report", jsonPath,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() code = %d, want 1\nstderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("expected json report: %v", err)
	}
	outBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if got := strings.Count(string(outBytes), "<trk>"); got != 2 {
		t.Fatalf("track count = %d, want 2", got)
	}
	if !strings.Contains(stdout.String(), "Files failed: 1") {
		t.Fatalf("summary missing failed count: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Failed files:\n- invalid.gpx (parse):") {
		t.Fatalf("summary missing failed file names: %s", stdout.String())
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

	outPath := filepath.Join(root, "out", "merged.gpx")
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

	outPath := filepath.Join(root, "out", "merged.gpx")
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
