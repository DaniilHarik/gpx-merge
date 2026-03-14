package processor

import (
	"strings"
	"testing"

	"gpx-merge/internal/gpx"
)

func seg(points []gpx.Point) gpx.Segment {
	return gpx.Segment{Points: points}
}

func timedSeg(times []string) gpx.Segment {
	pts := make([]gpx.Point, len(times))
	for i, t := range times {
		pts[i] = gpx.Point{Lat: float64(i), Lon: 0, Time: t}
	}
	return gpx.Segment{Points: pts}
}

func TestSortTrackSegmentsByFirstTimestamp_OutOfOrder(t *testing.T) {
	t.Parallel()
	tracks := []gpx.Track{{
		Name: "T",
		Segments: []gpx.Segment{
			timedSeg([]string{"2024-06-02T10:00:00Z", "2024-06-02T10:01:00Z"}),
			timedSeg([]string{"2024-06-01T08:00:00Z", "2024-06-01T08:01:00Z"}),
		},
	}}

	out, reordered := sortTrackSegmentsByFirstTimestamp(tracks)
	if reordered != 1 {
		t.Fatalf("reordered = %d, want 1", reordered)
	}
	first := out[0].Segments[0].Points[0].Time
	if first != "2024-06-01T08:00:00Z" {
		t.Fatalf("first segment time = %q, want 2024-06-01", first)
	}
}

func TestSortTrackSegmentsByFirstTimestamp_AlreadyOrdered(t *testing.T) {
	t.Parallel()
	tracks := []gpx.Track{{
		Name: "T",
		Segments: []gpx.Segment{
			timedSeg([]string{"2024-06-01T08:00:00Z", "2024-06-01T08:01:00Z"}),
			timedSeg([]string{"2024-06-02T10:00:00Z", "2024-06-02T10:01:00Z"}),
		},
	}}

	_, reordered := sortTrackSegmentsByFirstTimestamp(tracks)
	if reordered != 0 {
		t.Fatalf("reordered = %d, want 0 for already-sorted segments", reordered)
	}
}

func TestSortTrackSegmentsByFirstTimestamp_NoTimestamps(t *testing.T) {
	t.Parallel()
	tracks := []gpx.Track{{
		Name: "T",
		Segments: []gpx.Segment{
			seg([]gpx.Point{{Lat: 1}, {Lat: 2}}),
			seg([]gpx.Point{{Lat: 3}, {Lat: 4}}),
		},
	}}

	_, reordered := sortTrackSegmentsByFirstTimestamp(tracks)
	if reordered != 0 {
		t.Fatalf("reordered = %d, want 0 when segments have no timestamps", reordered)
	}
}

func TestSortTrackSegmentsByFirstTimestamp_SingleSegment(t *testing.T) {
	t.Parallel()
	tracks := []gpx.Track{{
		Name:     "T",
		Segments: []gpx.Segment{timedSeg([]string{"2024-06-01T08:00:00Z"})},
	}}

	_, reordered := sortTrackSegmentsByFirstTimestamp(tracks)
	if reordered != 0 {
		t.Fatalf("reordered = %d, want 0 for single segment", reordered)
	}
}

func TestSortTrackSegmentsByFirstTimestamp_EmptyTracks(t *testing.T) {
	t.Parallel()
	out, reordered := sortTrackSegmentsByFirstTimestamp(nil)
	if reordered != 0 {
		t.Fatalf("reordered = %d, want 0 for nil input", reordered)
	}
	if len(out) != 0 {
		t.Fatalf("len(out) = %d, want 0", len(out))
	}
}

func TestFileWarnings_LargeGap(t *testing.T) {
	t.Parallel()
	// Segments 20+ km apart (well above the 1000m threshold)
	tracks := []gpx.Track{{
		Name: "Gap",
		Segments: []gpx.Segment{
			seg([]gpx.Point{{Lat: 58.0, Lon: 24.0}, {Lat: 58.001, Lon: 24.001}}),
			seg([]gpx.Point{{Lat: 58.2, Lon: 24.2}, {Lat: 58.201, Lon: 24.201}}),
		},
	}}

	warnings := fileWarnings("gap.gpx", tracks, 0)
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1", len(warnings))
	}
	if warnings[0].Path != "gap.gpx" {
		t.Fatalf("Path = %q, want gap.gpx", warnings[0].Path)
	}
	if !strings.Contains(warnings[0].Message, "segment discontinuities") {
		t.Fatalf("Message = %q, want to contain 'segment discontinuities'", warnings[0].Message)
	}
}

func TestFileWarnings_SmallGap(t *testing.T) {
	t.Parallel()
	// Segments only meters apart (below the 1000m threshold)
	tracks := []gpx.Track{{
		Name: "Close",
		Segments: []gpx.Segment{
			seg([]gpx.Point{{Lat: 58.0, Lon: 24.0}, {Lat: 58.0001, Lon: 24.0001}}),
			seg([]gpx.Point{{Lat: 58.0002, Lon: 24.0002}, {Lat: 58.0003, Lon: 24.0003}}),
		},
	}}

	warnings := fileWarnings("close.gpx", tracks, 0)
	if len(warnings) != 0 {
		t.Fatalf("len(warnings) = %d, want 0 for small gap", len(warnings))
	}
}

func TestFileWarnings_SplitThresholdExceeded(t *testing.T) {
	t.Parallel()
	tracks := []gpx.Track{{
		Name: "Gap",
		Segments: []gpx.Segment{
			seg([]gpx.Point{{Lat: 58.0, Lon: 24.0}, {Lat: 58.001, Lon: 24.001}}),
			seg([]gpx.Point{{Lat: 58.2, Lon: 24.2}, {Lat: 58.201, Lon: 24.201}}),
		},
	}}

	warnings := fileWarnings("gap.gpx", tracks, 5000)
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1", len(warnings))
	}
	if !strings.Contains(warnings[0].Message, "split threshold") {
		t.Fatalf("Message = %q, want to mention split threshold", warnings[0].Message)
	}
}

func TestFileWarnings_ShortSegmentsSkipped(t *testing.T) {
	t.Parallel()
	// Segments with fewer than 2 points should be skipped in gap check
	tracks := []gpx.Track{{
		Name: "Short",
		Segments: []gpx.Segment{
			seg([]gpx.Point{{Lat: 58.0, Lon: 24.0}}),
			seg([]gpx.Point{{Lat: 59.0, Lon: 24.0}}),
		},
	}}

	warnings := fileWarnings("short.gpx", tracks, 0)
	if len(warnings) != 0 {
		t.Fatalf("len(warnings) = %d, want 0 for segments with <2 points", len(warnings))
	}
}
