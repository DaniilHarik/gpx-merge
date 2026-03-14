package gpx

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGPXFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestParseFile_NamedTrack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "ride.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1">
  <trk><name>Morning Ride</name><trkseg>
    <trkpt lat="58.0" lon="24.0"><ele>10.5</ele><time>2024-01-01T08:00:00Z</time></trkpt>
    <trkpt lat="58.1" lon="24.1"></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := ParseFile(p, "ride.gpx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(tracks))
	}
	if tracks[0].Name != "Morning Ride" {
		t.Fatalf("Name = %q, want %q", tracks[0].Name, "Morning Ride")
	}
	if len(tracks[0].Segments) != 1 {
		t.Fatalf("len(Segments) = %d, want 1", len(tracks[0].Segments))
	}
	pts := tracks[0].Segments[0].Points
	if len(pts) != 2 {
		t.Fatalf("len(Points) = %d, want 2", len(pts))
	}
	if pts[0].Lat != 58.0 || pts[0].Lon != 24.0 {
		t.Fatalf("pts[0] = {%.1f, %.1f}, want {58.0, 24.0}", pts[0].Lat, pts[0].Lon)
	}
	if pts[0].Ele == nil || *pts[0].Ele != 10.5 {
		t.Fatalf("pts[0].Ele = %v, want 10.5", pts[0].Ele)
	}
	if pts[0].Time != "2024-01-01T08:00:00Z" {
		t.Fatalf("pts[0].Time = %q, want RFC3339", pts[0].Time)
	}
	if pts[1].Ele != nil {
		t.Fatalf("pts[1].Ele = %v, want nil", pts[1].Ele)
	}
}

func TestParseFile_SingleUnnamedTrackUsesFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "my_ride.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1">
  <trk><trkseg>
    <trkpt lat="58.0" lon="24.0"></trkpt>
    <trkpt lat="58.1" lon="24.1"></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := ParseFile(p, "my_ride.gpx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracks[0].Name != "my_ride" {
		t.Fatalf("Name = %q, want %q", tracks[0].Name, "my_ride")
	}
}

func TestParseFile_MultipleUnnamedTracksNumbered(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "export.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1">
  <trk><trkseg>
    <trkpt lat="58.0" lon="24.0"></trkpt>
    <trkpt lat="58.1" lon="24.1"></trkpt>
  </trkseg></trk>
  <trk><trkseg>
    <trkpt lat="59.0" lon="25.0"></trkpt>
    <trkpt lat="59.1" lon="25.1"></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := ParseFile(p, "export.gpx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	if tracks[0].Name != "export #1" {
		t.Fatalf("tracks[0].Name = %q, want %q", tracks[0].Name, "export #1")
	}
	if tracks[1].Name != "export #2" {
		t.Fatalf("tracks[1].Name = %q, want %q", tracks[1].Name, "export #2")
	}
}

func TestParseFile_WhitespaceTrimmed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "trip.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1">
  <trk><name>  Padded Name  </name><trkseg><name>  Seg  </name>
    <trkpt lat="58.0" lon="24.0"><time>  2024-01-01T08:00:00Z  </time></trkpt>
    <trkpt lat="58.1" lon="24.1"></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := ParseFile(p, "trip.gpx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracks[0].Name != "Padded Name" {
		t.Fatalf("Name = %q, want trimmed", tracks[0].Name)
	}
	if tracks[0].Segments[0].Name != "Seg" {
		t.Fatalf("Segment.Name = %q, want trimmed", tracks[0].Segments[0].Name)
	}
	if tracks[0].Segments[0].Points[0].Time != "2024-01-01T08:00:00Z" {
		t.Fatalf("Point.Time = %q, want trimmed", tracks[0].Segments[0].Points[0].Time)
	}
}

func TestParseFile_NoTracks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "empty.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1"></gpx>`)

	_, err := ParseFile(p, "empty.gpx")
	if err == nil {
		t.Fatal("expected error for GPX with no tracks, got nil")
	}
}

func TestParseFile_MalformedXML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "bad.gpx", `<gpx><trk><unclosed>`)

	_, err := ParseFile(p, "bad.gpx")
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := ParseFile("/nonexistent/path/file.gpx", "file.gpx")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
