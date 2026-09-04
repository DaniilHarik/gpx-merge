package gpx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGPXFile(t testing.TB, dir, name, content string) string {
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

	tracks, err := ParseFile(context.Background(), p, "ride.gpx")
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

func TestParseFileWithOptionsSkipsUnneededPointFields(t *testing.T) {
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "ride.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1">
  <trk><trkseg>
    <trkpt lat="58.0" lon="24.0"><ele>10.5</ele><time>2024-01-01T08:00:00Z</time></trkpt>
    <trkpt lat="58.1" lon="24.1"><ele>11.5</ele><time>2024-01-01T08:01:00Z</time></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := ParseFileWithOptions(context.Background(), p, "ride.gpx", ParseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	points := tracks[0].Segments[0].Points
	for i, point := range points {
		if point.Ele != nil {
			t.Fatalf("point %d elevation = %v, want nil", i, point.Ele)
		}
		if point.Time != "" {
			t.Fatalf("point %d time = %q, want empty", i, point.Time)
		}
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

	tracks, err := ParseFile(context.Background(), p, "my_ride.gpx")
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

	tracks, err := ParseFile(context.Background(), p, "export.gpx")
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

	tracks, err := ParseFile(context.Background(), p, "trip.gpx")
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

func TestParseFile_SkipsUnknownElements(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "ride.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1">
  <metadata><extensions><ignored><nested>value</nested></ignored></extensions></metadata>
  <trk>
    <name>Ride</name>
    <extensions><ignored><nested>value</nested></ignored></extensions>
    <trkseg>
      <name>Seg</name>
      <extensions><ignored /></extensions>
      <trkpt lat="58.0" lon="24.0"><extensions><ignored /></extensions><time>2024-01-01T08:00:00Z</time></trkpt>
    </trkseg>
  </trk>
</gpx>`)

	tracks, err := ParseFile(context.Background(), p, "ride.gpx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(tracks))
	}
	if tracks[0].Name != "Ride" {
		t.Fatalf("Name = %q, want Ride", tracks[0].Name)
	}
	if got := tracks[0].Segments[0].Points[0].Time; got != "2024-01-01T08:00:00Z" {
		t.Fatalf("Time = %q, want timestamp", got)
	}
}

func TestParseFile_NamespacedElementsUseLocalNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "ride.gpx", `<?xml version="1.0"?>
<x:gpx xmlns:x="http://www.topografix.com/GPX/1/1">
  <x:trk><x:name>Ride</x:name><x:trkseg>
    <x:trkpt lat="58.0" lon="24.0"><x:ele>10.5</x:ele></x:trkpt>
  </x:trkseg></x:trk>
</x:gpx>`)

	tracks, err := ParseFile(context.Background(), p, "ride.gpx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracks[0].Name != "Ride" {
		t.Fatalf("Name = %q, want Ride", tracks[0].Name)
	}
	if ele := tracks[0].Segments[0].Points[0].Ele; ele == nil || *ele != 10.5 {
		t.Fatalf("Ele = %v, want 10.5", ele)
	}
}

func TestParseFile_NoTracks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "empty.gpx", `<?xml version="1.0"?>
<gpx xmlns="http://www.topografix.com/GPX/1/1"></gpx>`)

	_, err := ParseFile(context.Background(), p, "empty.gpx")
	if err == nil {
		t.Fatal("expected error for GPX with no tracks, got nil")
	}
}

func TestParseFile_MalformedXML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "bad.gpx", `<gpx><trk><unclosed>`)

	_, err := ParseFile(context.Background(), p, "bad.gpx")
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

func TestParseFile_MismatchedUnknownElement(t *testing.T) {
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "bad.gpx", `<gpx><extensions><value></other></extensions><trk><trkseg><trkpt lat="58" lon="24"/><trkpt lat="58.1" lon="24.1"/></trkseg></trk></gpx>`)

	_, err := ParseFile(context.Background(), p, "bad.gpx")
	if err == nil || !strings.Contains(err.Error(), "unexpected closing element") {
		t.Fatalf("err = %v, want a mismatched element error", err)
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := ParseFile(context.Background(), "/nonexistent/path/file.gpx", "file.gpx")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseFile_InvalidCoordinates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cases := []struct {
		name    string
		lat     string
		lon     string
		wantErr string
	}{
		{"lat too high", "91.0", "24.0", "invalid latitude"},
		{"lat too low", "-91.0", "24.0", "invalid latitude"},
		{"lon too high", "58.0", "181.0", "invalid longitude"},
		{"lon too low", "58.0", "-181.0", "invalid longitude"},
		{"lat exactly 90 is valid", "90.0", "0.0", ""},
		{"lat exactly -90 is valid", "-90.0", "0.0", ""},
		{"lon exactly 180 is valid", "0.0", "180.0", ""},
		{"lon exactly -180 is valid", "0.0", "-180.0", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			content := `<?xml version="1.0"?>` +
				`<gpx xmlns="http://www.topografix.com/GPX/1/1">` +
				`<trk><trkseg>` +
				`<trkpt lat="` + tc.lat + `" lon="` + tc.lon + `"></trkpt>` +
				`<trkpt lat="58.1" lon="24.1"></trkpt>` +
				`</trkseg></trk></gpx>`
			p := writeGPXFile(t, dir, tc.name+".gpx", content)
			_, err := ParseFile(context.Background(), p, tc.name+".gpx")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
			}
		})
	}
}

func TestParseFile_CanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ParseFile(ctx, "/nonexistent/path/file.gpx", "file.gpx")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestParseFile_CanceledDuringDecode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeGPXFile(t, dir, "ride.gpx", validParseGPX(100))

	ctx := &parseStepCancelContext{Context: context.Background(), cancelOnCall: 4}
	_, err := ParseFile(ctx, p, "ride.gpx")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func BenchmarkParseFile(b *testing.B) {
	dir := b.TempDir()
	p := writeGPXFile(b, dir, "ride.gpx", validParseGPX(20000))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracks, err := ParseFile(context.Background(), p, "ride.gpx")
		if err != nil {
			b.Fatalf("ParseFile: %v", err)
		}
		if len(tracks) != 1 {
			b.Fatalf("len(tracks) = %d, want 1", len(tracks))
		}
	}
}

func validParseGPX(nPoints int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1"><trk><name>Ride</name><trkseg>`)
	for i := 0; i < nPoints; i++ {
		sb.WriteString(`<trkpt lat="58.0" lon="24.0"></trkpt>`)
	}
	sb.WriteString(`</trkseg></trk></gpx>`)
	return sb.String()
}

type parseStepCancelContext struct {
	context.Context
	cancelOnCall int
	errCalls     int
}

func (c *parseStepCancelContext) Err() error {
	c.errCalls++
	if c.errCalls >= c.cancelOnCall {
		return context.Canceled
	}
	return nil
}
