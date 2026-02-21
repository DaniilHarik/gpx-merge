package optimize

import "testing"

func TestSimplifyPreservesEndpoints(t *testing.T) {
	t.Parallel()
	pts := []Coord{{0, 0}, {0.00001, 0.00001}, {0.00002, 0.00002}, {0.00003, 0.00003}}
	res := SimplifyIndices(pts, Options{SimplifyMeters: 2.0, MaxErrorMeters: 5.0, MinPoints: 2})
	if len(res.Indexes) < 2 {
		t.Fatalf("unexpected index count: %d", len(res.Indexes))
	}
	if res.Indexes[0] != 0 {
		t.Fatalf("first index = %d", res.Indexes[0])
	}
	if res.Indexes[len(res.Indexes)-1] != len(pts)-1 {
		t.Fatalf("last index = %d", res.Indexes[len(res.Indexes)-1])
	}
}

func TestSimplifyRespectsMaxError(t *testing.T) {
	t.Parallel()
	pts := []Coord{{0, 0}, {0.00001, 0.00007}, {0.00002, 0}, {0.00003, 0.00007}, {0.00004, 0}}
	res := SimplifyIndices(pts, Options{SimplifyMeters: 20, MaxErrorMeters: 2, MinPoints: 2})
	if res.MaxDeviation > 2.000001 {
		t.Fatalf("MaxDeviation = %.4f, want <= 2", res.MaxDeviation)
	}
}

func TestSimplifyEnforcesMinPoints(t *testing.T) {
	t.Parallel()
	pts := []Coord{{0, 0}, {0.00001, 0.00001}, {0.00002, 0.00002}, {0.00003, 0.00003}, {0.00004, 0.00004}}
	res := SimplifyIndices(pts, Options{SimplifyMeters: 50, MaxErrorMeters: 50, MinPoints: 4})
	if len(res.Indexes) < 4 {
		t.Fatalf("len(indexes) = %d, want >=4", len(res.Indexes))
	}
}

func TestSimplifyTinySegmentIdempotent(t *testing.T) {
	t.Parallel()
	pts := []Coord{{58.1, 24.1}, {58.2, 24.2}}
	res := SimplifyIndices(pts, Options{SimplifyMeters: 5, MaxErrorMeters: 5, MinPoints: 2})
	if len(res.Indexes) != 2 || res.Indexes[0] != 0 || res.Indexes[1] != 1 {
		t.Fatalf("indexes = %v", res.Indexes)
	}
}
