package processor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"gpx-merge/internal/gpx"
	"gpx-merge/internal/pipeline"
	"gpx-merge/internal/report"
)

func makeSuccessResult(index int, relPath string, payload filePayload) pipeline.Result {
	return pipeline.Result{
		File:     pipeline.File{Index: index, RelPath: relPath},
		Payload:  payload,
		Duration: time.Millisecond,
	}
}

func makeErrorResult(index int, relPath string, err error) pipeline.Result {
	return pipeline.Result{
		File:     pipeline.File{Index: index, RelPath: relPath},
		Err:      err,
		Duration: time.Millisecond,
	}
}

func TestAggregateResults_AllSuccess(t *testing.T) {
	t.Parallel()
	results := []pipeline.Result{
		makeSuccessResult(0, "a.gpx", filePayload{
			Tracks:       []gpx.Track{{Name: "A"}},
			PointsIn:     100,
			PointsOut:    50,
			BytesIn:      1000,
			BytesOut:     500,
			DistanceInM:  10000,
			DistanceOutM: 9900,
		}),
		makeSuccessResult(1, "b.gpx", filePayload{
			Tracks:       []gpx.Track{{Name: "B"}},
			PointsIn:     200,
			PointsOut:    80,
			BytesIn:      2000,
			BytesOut:     900,
			DistanceInM:  20000,
			DistanceOutM: 19800,
		}),
	}

	agg := AggregateResults(results, 2, false, &bytes.Buffer{})

	if agg.Totals.FilesProcessed != 2 {
		t.Fatalf("FilesProcessed = %d, want 2", agg.Totals.FilesProcessed)
	}
	if agg.Totals.FilesFailed != 0 {
		t.Fatalf("FilesFailed = %d, want 0", agg.Totals.FilesFailed)
	}
	if agg.Totals.FilesScanned != 2 {
		t.Fatalf("FilesScanned = %d, want 2", agg.Totals.FilesScanned)
	}
	if agg.Totals.PointsIn != 300 {
		t.Fatalf("PointsIn = %d, want 300", agg.Totals.PointsIn)
	}
	if agg.Totals.PointsOut != 130 {
		t.Fatalf("PointsOut = %d, want 130", agg.Totals.PointsOut)
	}
	if agg.Totals.BytesIn != 3000 {
		t.Fatalf("BytesIn = %d, want 3000", agg.Totals.BytesIn)
	}
	if agg.Totals.DistanceInM != 30000 {
		t.Fatalf("DistanceInM = %f, want 30000", agg.Totals.DistanceInM)
	}
	if len(agg.AllTracks) != 2 {
		t.Fatalf("len(AllTracks) = %d, want 2", len(agg.AllTracks))
	}
	if len(agg.ErrorsOut) != 0 {
		t.Fatalf("len(ErrorsOut) = %d, want 0", len(agg.ErrorsOut))
	}
	if len(agg.FileStats) != 2 {
		t.Fatalf("len(FileStats) = %d, want 2", len(agg.FileStats))
	}
	for _, s := range agg.FileStats {
		if s.Status != "ok" {
			t.Fatalf("FileStats[%s].Status = %q, want ok", s.Path, s.Status)
		}
	}
}

func TestAggregateResults_AllErrors(t *testing.T) {
	t.Parallel()
	results := []pipeline.Result{
		makeErrorResult(0, "a.gpx", &fileError{Stage: "parse", Err: errors.New("unexpected EOF")}),
		makeErrorResult(1, "b.gpx", &fileError{Stage: "stat", Err: errors.New("no such file")}),
	}

	agg := AggregateResults(results, 2, false, &bytes.Buffer{})

	if agg.Totals.FilesFailed != 2 {
		t.Fatalf("FilesFailed = %d, want 2", agg.Totals.FilesFailed)
	}
	if agg.Totals.FilesProcessed != 0 {
		t.Fatalf("FilesProcessed = %d, want 0", agg.Totals.FilesProcessed)
	}
	if len(agg.AllTracks) != 0 {
		t.Fatalf("len(AllTracks) = %d, want 0", len(agg.AllTracks))
	}
	if len(agg.ErrorsOut) != 2 {
		t.Fatalf("len(ErrorsOut) = %d, want 2", len(agg.ErrorsOut))
	}
	if agg.ErrorsOut[0].Stage != "parse" {
		t.Fatalf("ErrorsOut[0].Stage = %q, want parse", agg.ErrorsOut[0].Stage)
	}
	if agg.ErrorsOut[1].Stage != "stat" {
		t.Fatalf("ErrorsOut[1].Stage = %q, want stat", agg.ErrorsOut[1].Stage)
	}
	for _, s := range agg.FileStats {
		if s.Status != "error" {
			t.Fatalf("FileStats[%s].Status = %q, want error", s.Path, s.Status)
		}
	}
}

func TestAggregateResults_Mixed(t *testing.T) {
	t.Parallel()
	results := []pipeline.Result{
		makeSuccessResult(0, "good.gpx", filePayload{
			Tracks:   []gpx.Track{{Name: "Good"}},
			PointsIn: 10,
			PointsOut: 5,
		}),
		makeErrorResult(1, "bad.gpx", &fileError{Stage: "parse", Err: errors.New("bad xml")}),
	}

	agg := AggregateResults(results, 2, false, &bytes.Buffer{})

	if agg.Totals.FilesProcessed != 1 {
		t.Fatalf("FilesProcessed = %d, want 1", agg.Totals.FilesProcessed)
	}
	if agg.Totals.FilesFailed != 1 {
		t.Fatalf("FilesFailed = %d, want 1", agg.Totals.FilesFailed)
	}
	if len(agg.AllTracks) != 1 {
		t.Fatalf("len(AllTracks) = %d, want 1", len(agg.AllTracks))
	}
	if len(agg.ErrorsOut) != 1 {
		t.Fatalf("len(ErrorsOut) = %d, want 1", len(agg.ErrorsOut))
	}
}

func TestAggregateResults_WrongPayloadType(t *testing.T) {
	t.Parallel()
	results := []pipeline.Result{
		{
			File:    pipeline.File{Index: 0, RelPath: "x.gpx"},
			Payload: "not a filePayload",
		},
	}

	agg := AggregateResults(results, 1, false, &bytes.Buffer{})

	if agg.Totals.FilesFailed != 1 {
		t.Fatalf("FilesFailed = %d, want 1", agg.Totals.FilesFailed)
	}
	if len(agg.ErrorsOut) != 1 {
		t.Fatalf("len(ErrorsOut) = %d, want 1", len(agg.ErrorsOut))
	}
	if agg.ErrorsOut[0].Stage != "internal" {
		t.Fatalf("ErrorsOut[0].Stage = %q, want internal", agg.ErrorsOut[0].Stage)
	}
}

func TestAggregateResults_NonFileErrorUsesProcessStage(t *testing.T) {
	t.Parallel()
	results := []pipeline.Result{
		makeErrorResult(0, "a.gpx", errors.New("generic error")),
	}

	agg := AggregateResults(results, 1, false, &bytes.Buffer{})

	if agg.ErrorsOut[0].Stage != "process" {
		t.Fatalf("Stage = %q, want process", agg.ErrorsOut[0].Stage)
	}
}

func TestAggregateResults_VerboseWritesToStdout(t *testing.T) {
	t.Parallel()
	results := []pipeline.Result{
		makeSuccessResult(0, "v.gpx", filePayload{
			Tracks:    []gpx.Track{{Name: "V"}},
			PointsIn:  10,
			PointsOut: 5,
		}),
		makeErrorResult(1, "e.gpx", &fileError{Stage: "parse", Err: errors.New("bad")}),
	}

	var buf bytes.Buffer
	AggregateResults(results, 2, true, &buf)

	out := buf.String()
	if !strings.Contains(out, "[ok] v.gpx") {
		t.Fatalf("verbose output missing [ok] line: %s", out)
	}
	if !strings.Contains(out, "[error] e.gpx") {
		t.Fatalf("verbose output missing [error] line: %s", out)
	}
}

func TestAggregateResults_WarningsCollected(t *testing.T) {
	t.Parallel()
	results := []pipeline.Result{
		makeSuccessResult(0, "w.gpx", filePayload{
			Tracks:    []gpx.Track{{Name: "W"}},
			PointsIn:  10,
			PointsOut: 5,
			Warnings: []report.WarningItem{
				{Path: "w.gpx", Message: "test warning"},
			},
		}),
	}

	agg := AggregateResults(results, 1, false, &bytes.Buffer{})

	if len(agg.WarningsOut) != 1 {
		t.Fatalf("len(WarningsOut) = %d, want 1", len(agg.WarningsOut))
	}
	if agg.WarningsOut[0].Message != "test warning" {
		t.Fatalf("WarningsOut[0].Message = %q", agg.WarningsOut[0].Message)
	}
}
