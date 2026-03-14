package processor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gpx-merge/internal/cli"
	"gpx-merge/internal/optimize"
	"gpx-merge/internal/pool"
)

func defaultConfig() cli.Config {
	return cli.Config{
		SimplifyMeters:      0.5,
		MaxErrorMeters:      1.5,
		MinPoints:           2,
		Precision:           6,
		SplitTrackGapMeters: 1000,
	}
}

func defaultOptOpts() optimize.Options {
	return optimize.Options{
		SimplifyMeters: 0.5,
		MaxErrorMeters: 1.5,
		MinPoints:      2,
	}
}

func writeTestGPX(t *testing.T, dir, filename, content string) pool.File {
	t.Helper()
	abs := filepath.Join(dir, filename)
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	return pool.File{Index: 0, RelPath: filename, AbsPath: abs}
}

func validGPX(name string, nPoints int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1"><trk><name>`)
	sb.WriteString(name)
	sb.WriteString(`</name><trkseg>`)
	for i := 0; i < nPoints; i++ {
		sb.WriteString(fmt.Sprintf(`<trkpt lat="%.6f" lon="24.0"></trkpt>`, 58.0+float64(i)*0.001))
	}
	sb.WriteString(`</trkseg></trk></gpx>`)
	return sb.String()
}

func TestProcess_CanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := NewFileProcessor(defaultConfig(), defaultOptOpts())
	_, err := p.Process(ctx, pool.File{Index: 0, RelPath: "any.gpx", AbsPath: "/any.gpx"})

	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	var fe *fileError
	if !errors.As(err, &fe) {
		t.Fatalf("expected fileError, got %T", err)
	}
	if fe.Stage != "canceled" {
		t.Fatalf("Stage = %q, want canceled", fe.Stage)
	}
}

func TestProcess_FileNotFound(t *testing.T) {
	t.Parallel()
	p := NewFileProcessor(defaultConfig(), defaultOptOpts())
	_, err := p.Process(context.Background(), pool.File{
		Index:   0,
		RelPath: "missing.gpx",
		AbsPath: "/nonexistent/path/missing.gpx",
	})

	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	var fe *fileError
	if !errors.As(err, &fe) {
		t.Fatalf("expected fileError, got %T", err)
	}
	if fe.Stage != "stat" {
		t.Fatalf("Stage = %q, want stat", fe.Stage)
	}
}

func TestProcess_InvalidGPX(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := writeTestGPX(t, dir, "bad.gpx", `<gpx><trk><unclosed>`)

	p := NewFileProcessor(defaultConfig(), defaultOptOpts())
	_, err := p.Process(context.Background(), f)

	if err == nil {
		t.Fatal("expected error for invalid GPX, got nil")
	}
	var fe *fileError
	if !errors.As(err, &fe) {
		t.Fatalf("expected fileError, got %T", err)
	}
	if fe.Stage != "parse" {
		t.Fatalf("Stage = %q, want parse", fe.Stage)
	}
}

func TestProcess_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := writeTestGPX(t, dir, "ride.gpx", validGPX("Morning Ride", 40))

	p := NewFileProcessor(defaultConfig(), defaultOptOpts())
	result, err := p.Process(context.Background(), f)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload, ok := result.(filePayload)
	if !ok {
		t.Fatalf("expected filePayload, got %T", result)
	}
	if len(payload.Tracks) == 0 {
		t.Fatal("expected at least one track")
	}
	if payload.PointsIn != 40 {
		t.Fatalf("PointsIn = %d, want 40", payload.PointsIn)
	}
	if payload.PointsOut < 2 || payload.PointsOut > 40 {
		t.Fatalf("PointsOut = %d, want 2..40", payload.PointsOut)
	}
	if payload.BytesIn <= 0 {
		t.Fatalf("BytesIn = %d, want > 0", payload.BytesIn)
	}
	if payload.BytesOut <= 0 {
		t.Fatalf("BytesOut = %d, want > 0", payload.BytesOut)
	}
}

func TestProcess_ErrorUnwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner cause")
	fe := &fileError{Stage: "parse", Path: "x.gpx", Err: inner}

	if !errors.Is(fe, inner) {
		t.Fatal("errors.Is should find inner error through Unwrap")
	}
	if !strings.Contains(fe.Error(), "parse") {
		t.Fatalf("Error() = %q, want to contain stage", fe.Error())
	}
}
