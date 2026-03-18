package processor

import (
	"context"
	"errors"
	"testing"

	"gpx-merge/internal/gpx"
	"gpx-merge/internal/optimize"
)

var defaultOpts = optimize.Options{
	SimplifyMeters: 0.5,
	MaxErrorMeters: 1.5,
	MinPoints:      2,
}

func pts(coords [][2]float64) []gpx.Point {
	out := make([]gpx.Point, len(coords))
	for i, c := range coords {
		out[i] = gpx.Point{Lat: c[0], Lon: c[1]}
	}
	return out
}

func straightLine(n int) []gpx.Point {
	coords := make([][2]float64, n)
	for i := range coords {
		coords[i] = [2]float64{58.0 + float64(i)*0.001, 24.0}
	}
	return pts(coords)
}

func TestOptimizeTrack_NormalPath(t *testing.T) {
	t.Parallel()
	track := gpx.Track{
		Name: "Test",
		Segments: []gpx.Segment{
			{Points: straightLine(50)},
		},
	}

	out, pointsIn, pointsOut, distIn, distOut, err := optimizeTrack(context.Background(), track, defaultOpts, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Name != "Test" {
		t.Fatalf("Name = %q, want Test", out.Name)
	}
	if pointsIn != 50 {
		t.Fatalf("pointsIn = %d, want 50", pointsIn)
	}
	if pointsOut < 2 || pointsOut > pointsIn {
		t.Fatalf("pointsOut = %d, want 2..50", pointsOut)
	}
	if distIn <= 0 {
		t.Fatalf("distIn = %f, want > 0", distIn)
	}
	if distOut <= 0 || distOut > distIn {
		t.Fatalf("distOut = %f, want 0..%f", distOut, distIn)
	}
}

func TestOptimizeTrack_SegmentTooFewPoints(t *testing.T) {
	t.Parallel()
	track := gpx.Track{
		Name: "Bad",
		Segments: []gpx.Segment{
			{Points: pts([][2]float64{{58.0, 24.0}})},
		},
	}

	_, _, _, _, _, err := optimizeTrack(context.Background(), track, defaultOpts, false, false)
	if err == nil {
		t.Fatal("expected error for segment with 1 point, got nil")
	}
}

func TestOptimizeTrack_KeepEleStripsEleWhenFalse(t *testing.T) {
	t.Parallel()
	ele := 100.0
	track := gpx.Track{
		Name: "Ele",
		Segments: []gpx.Segment{{
			Points: []gpx.Point{
				{Lat: 58.0, Lon: 24.0, Ele: &ele},
				{Lat: 58.1, Lon: 24.1, Ele: &ele},
				{Lat: 58.2, Lon: 24.2, Ele: &ele},
			},
		}},
	}

	out, _, _, _, _, err := optimizeTrack(context.Background(), track, defaultOpts, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, seg := range out.Segments {
		for _, p := range seg.Points {
			if p.Ele != nil {
				t.Fatalf("expected Ele=nil when keepEle=false, got %v", p.Ele)
			}
		}
	}
}

func TestOptimizeTrack_KeepElePreservesEleWhenTrue(t *testing.T) {
	t.Parallel()
	ele := 100.0
	track := gpx.Track{
		Name: "Ele",
		Segments: []gpx.Segment{{
			Points: []gpx.Point{
				{Lat: 58.0, Lon: 24.0, Ele: &ele},
				{Lat: 58.1, Lon: 24.1, Ele: &ele},
				{Lat: 58.2, Lon: 24.2, Ele: &ele},
			},
		}},
	}

	out, _, _, _, _, err := optimizeTrack(context.Background(), track, defaultOpts, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, seg := range out.Segments {
		for _, p := range seg.Points {
			if p.Ele == nil {
				t.Fatal("expected Ele to be preserved when keepEle=true")
			}
		}
	}
}

func TestOptimizeTrack_KeepTimeStripsTimeWhenFalse(t *testing.T) {
	t.Parallel()
	track := gpx.Track{
		Name: "Time",
		Segments: []gpx.Segment{{
			Points: []gpx.Point{
				{Lat: 58.0, Lon: 24.0, Time: "2024-01-01T00:00:00Z"},
				{Lat: 58.1, Lon: 24.1, Time: "2024-01-01T00:01:00Z"},
				{Lat: 58.2, Lon: 24.2, Time: "2024-01-01T00:02:00Z"},
			},
		}},
	}

	out, _, _, _, _, err := optimizeTrack(context.Background(), track, defaultOpts, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, seg := range out.Segments {
		for _, p := range seg.Points {
			if p.Time != "" {
				t.Fatalf("expected Time=%q when keepTime=false, got %q", "", p.Time)
			}
		}
	}
}

func TestOptimizeTrack_MultipleSegments(t *testing.T) {
	t.Parallel()
	track := gpx.Track{
		Name: "Multi",
		Segments: []gpx.Segment{
			{Points: straightLine(20)},
			{Points: straightLine(30)},
		},
	}

	out, pointsIn, _, _, _, err := optimizeTrack(context.Background(), track, defaultOpts, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2", len(out.Segments))
	}
	if pointsIn != 50 {
		t.Fatalf("pointsIn = %d, want 50", pointsIn)
	}
}

func TestOptimizeTrack_ContextCanceledBeforeWork(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	track := gpx.Track{
		Name: "Canceled",
		Segments: []gpx.Segment{
			{Points: straightLine(20)},
		},
	}

	_, _, _, _, _, err := optimizeTrack(ctx, track, defaultOpts, false, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestOptimizeTrack_ContextCanceledBetweenSegments(t *testing.T) {
	t.Parallel()
	ctx := &stepCancelContext{Context: context.Background(), cancelOnCall: 2}
	track := gpx.Track{
		Name: "Canceled",
		Segments: []gpx.Segment{
			{Points: straightLine(20)},
			{Points: straightLine(20)},
		},
	}

	_, _, _, _, _, err := optimizeTrack(ctx, track, defaultOpts, false, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestSegmentDistanceMeters_TwoPoints(t *testing.T) {
	t.Parallel()
	points := []gpx.Point{
		{Lat: 58.0, Lon: 24.0},
		{Lat: 59.0, Lon: 24.0},
	}
	d := segmentDistanceMeters(points)
	// ~111 km between 1 degree of latitude
	if d < 100_000 || d > 120_000 {
		t.Fatalf("distance = %f, want ~111000", d)
	}
}

func TestSegmentDistanceMeters_FewerThanTwoPoints(t *testing.T) {
	t.Parallel()
	if got := segmentDistanceMeters(nil); got != 0 {
		t.Fatalf("distance = %f, want 0 for nil", got)
	}
	if got := segmentDistanceMeters([]gpx.Point{{Lat: 58.0, Lon: 24.0}}); got != 0 {
		t.Fatalf("distance = %f, want 0 for 1 point", got)
	}
}

type stepCancelContext struct {
	context.Context
	cancelOnCall int
	errCalls     int
}

func (c *stepCancelContext) Err() error {
	c.errCalls++
	if c.errCalls >= c.cancelOnCall {
		return context.Canceled
	}
	return nil
}
