package gpx

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestWriteMergedPartsMatchesTracks(t *testing.T) {
	ele := 12.34
	tracks := []Track{
		{Name: `A & <B> "C"`, Segments: []Segment{
			{Name: "segment & one", Points: []Point{{Lat: 58.1234567, Lon: 24.7654321, Ele: &ele, Time: "a & b"}, {Lat: 58.1, Lon: 24.1}}},
			{Points: []Point{{Lat: 59, Lon: 25}, {Lat: 59.1, Lon: 25.1}}},
		}},
		{Name: "second", Segments: []Segment{{Points: []Point{{Lat: 1, Lon: 2}, {Lat: 3, Lon: 4}}}}},
		{Name: "empty"},
	}
	for _, opts := range []WriteOptions{
		{Precision: 6},
		{Precision: 3, KeepEle: true, KeepTime: true, SplitTrackGapMeters: 1000},
		{Precision: 8, Creator: `creator & "name"`, IncludeMetadata: true, MetadataDesc: "a < b", MetadataTime: time.Unix(0, 0)},
	} {
		var parts [][]byte
		for _, track := range tracks {
			part, err := EncodeTracks([]Track{track}, opts)
			if err != nil {
				t.Fatal(err)
			}
			measured, err := MeasureTracks([]Track{track}, opts)
			if err != nil || measured != int64(len(part)) {
				t.Fatalf("fragment length %d != measured %d: %v", len(part), measured, err)
			}
			parts = append(parts, part)
		}
		var want, got bytes.Buffer
		if _, err := WriteMerged(&want, tracks, opts); err != nil {
			t.Fatal(err)
		}
		n, err := WriteMergedParts(&got, parts, opts)
		if err != nil || n != int64(want.Len()) || !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Fatalf("merged parts differ: length=%d err=%v\ngot=%s\nwant=%s", n, err, &got, &want)
		}
		measured, err := WriteMergedParts(io.Discard, parts, opts)
		if err != nil || measured != n {
			t.Fatalf("dry-run length %d != written %d: %v", measured, n, err)
		}
	}
}

func TestWriteMergedPartsReportsWriteFailure(t *testing.T) {
	failure := errors.New("disk full")
	for _, size := range []int{10, 512 * 1024} {
		writer := &limitedWriter{remaining: 42, err: failure}
		n, err := WriteMergedParts(writer, [][]byte{[]byte(strings.Repeat("x", size))}, WriteOptions{})
		if !errors.Is(err, failure) || n != 42 {
			t.Fatalf("size=%d: wrote %d, error %v; want 42 and disk full", size, n, err)
		}
	}
}

type limitedWriter struct {
	remaining int
	err       error
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	n := min(w.remaining, len(p))
	w.remaining -= n
	if n != len(p) {
		return n, w.err
	}
	return n, nil
}
