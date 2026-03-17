package processor

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"gpx-merge/internal/gpx"
	"gpx-merge/internal/pool"
	"gpx-merge/internal/report"
)

func makeSuccessResult(index int, relPath string, payload filePayload) pool.Result {
	return pool.Result{
		File:     pool.File{Index: index, RelPath: relPath},
		Payload:  payload,
		Duration: time.Millisecond,
	}
}

func TestAggregateResults_AllSuccess(t *testing.T) {
	t.Parallel()
	results := []pool.Result{
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
	if len(agg.FileStats) != 2 {
		t.Fatalf("len(FileStats) = %d, want 2", len(agg.FileStats))
	}
	for _, s := range agg.FileStats {
		if s.Status != "ok" {
			t.Fatalf("FileStats[%s].Status = %q, want ok", s.Path, s.Status)
		}
	}
}

func TestAggregateResults_WrongPayloadType(t *testing.T) {
	t.Parallel()
	results := []pool.Result{
		{
			File:    pool.File{Index: 0, RelPath: "x.gpx"},
			Payload: "not a filePayload",
		},
	}

	agg := AggregateResults(results, 1, false, &bytes.Buffer{})

	if agg.Totals.FilesFailed != 1 {
		t.Fatalf("FilesFailed = %d, want 1", agg.Totals.FilesFailed)
	}
	if len(agg.FileStats) != 1 || agg.FileStats[0].Stage != "internal" {
		t.Fatalf("expected internal error stat")
	}
}

func TestAggregateResults_VerboseWritesToStdout(t *testing.T) {
	t.Parallel()
	results := []pool.Result{
		makeSuccessResult(0, "v.gpx", filePayload{
			Tracks:    []gpx.Track{{Name: "V"}},
			PointsIn:  10,
			PointsOut: 5,
		}),
	}

	var buf bytes.Buffer
	AggregateResults(results, 1, true, &buf)

	out := buf.String()
	if !strings.Contains(out, "[ok] v.gpx") {
		t.Fatalf("verbose output missing [ok] line: %s", out)
	}
}

func TestAggregateResults_WarningsCollected(t *testing.T) {
	t.Parallel()
	results := []pool.Result{
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
