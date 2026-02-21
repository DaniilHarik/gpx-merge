package gpx

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteMergedGolden(t *testing.T) {
	t.Parallel()
	ele := 12.34
	tracks := []Track{
		{
			Name: "Track A",
			Segments: []Segment{{
				Name:   "Seg 1",
				Points: []Point{{Lat: 58.1234567, Lon: 24.7654321, Ele: &ele, Time: "2024-01-01T00:00:00Z"}, {Lat: 58.1234999, Lon: 24.7654999, Ele: &ele, Time: "2024-01-01T00:01:00Z"}},
			}},
		},
	}

	var buf bytes.Buffer
	_, err := WriteMerged(&buf, tracks, WriteOptions{
		Creator:         "gpx-merge/test",
		Precision:       6,
		KeepEle:         true,
		KeepTime:        true,
		IncludeMetadata: true,
		MetadataDesc:    "files_scanned=1 files_processed=1 files_failed=0 points_in=2 points_out=2",
		MetadataTime:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("WriteMerged() error = %v", err)
	}

	got := buf.String()
	expected := `<?xml version="1.0" encoding="UTF-8"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1" creator="gpx-merge/test"><metadata><time>2024-01-01T12:00:00Z</time><desc>files_scanned=1 files_processed=1 files_failed=0 points_in=2 points_out=2</desc></metadata><trk><name>Track A</name><trkseg><name>Seg 1</name><trkpt lat="58.123457" lon="24.765432"><ele>12.34</ele><time>2024-01-01T00:00:00Z</time></trkpt><trkpt lat="58.1235" lon="24.7655"><ele>12.34</ele><time>2024-01-01T00:01:00Z</time></trkpt></trkseg></trk></gpx>`
	if strings.TrimSpace(got) != strings.TrimSpace(expected) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, expected)
	}
}

func TestWriteMergedSplitTrackGap(t *testing.T) {
	t.Parallel()
	tracks := []Track{
		{
			Name: "Track A",
			Segments: []Segment{
				{
					Points: []Point{
						{Lat: 58.0, Lon: 24.0},
						{Lat: 58.0001, Lon: 24.0001},
					},
				},
				{
					Points: []Point{
						{Lat: 58.2, Lon: 24.2},
						{Lat: 58.2001, Lon: 24.2001},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	_, err := WriteMerged(&buf, tracks, WriteOptions{
		Creator:             "gpx-merge/test",
		Precision:           6,
		SplitTrackGapMeters: 1000,
	})
	if err != nil {
		t.Fatalf("WriteMerged() error = %v", err)
	}

	got := buf.String()
	if gotCount := strings.Count(got, "<trk>"); gotCount != 2 {
		t.Fatalf("track count = %d, want 2", gotCount)
	}
	if !strings.Contains(got, "<name>Track A (part 1)</name>") {
		t.Fatalf("missing part 1 track name: %s", got)
	}
	if !strings.Contains(got, "<name>Track A (part 2)</name>") {
		t.Fatalf("missing part 2 track name: %s", got)
	}
}

func TestWriteMergedSplitTrackGapDisabled(t *testing.T) {
	t.Parallel()
	tracks := []Track{
		{
			Name: "Track A",
			Segments: []Segment{
				{
					Points: []Point{
						{Lat: 58.0, Lon: 24.0},
						{Lat: 58.0001, Lon: 24.0001},
					},
				},
				{
					Points: []Point{
						{Lat: 58.2, Lon: 24.2},
						{Lat: 58.2001, Lon: 24.2001},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	_, err := WriteMerged(&buf, tracks, WriteOptions{
		Creator:             "gpx-merge/test",
		Precision:           6,
		SplitTrackGapMeters: 0,
	})
	if err != nil {
		t.Fatalf("WriteMerged() error = %v", err)
	}

	got := buf.String()
	if gotCount := strings.Count(got, "<trk>"); gotCount != 1 {
		t.Fatalf("track count = %d, want 1", gotCount)
	}
	if strings.Contains(got, "(part 1)") || strings.Contains(got, "(part 2)") {
		t.Fatalf("unexpected split names when disabled: %s", got)
	}
}
