package optimize

import (
	"math"
	"testing"
)

func BenchmarkSimplifyIndices(b *testing.B) {
	pts := make([]Coord, 20000)
	for i := range pts {
		x := float64(i) / 10000.0
		y := math.Sin(float64(i)/200.0) * 0.0005
		pts[i] = Coord{Lat: 58.0 + y, Lon: 24.0 + x*0.001}
	}

	opts := Options{SimplifyMeters: 1.5, MaxErrorMeters: 3.0, MinPoints: 2}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SimplifyIndices(pts, opts)
	}
}
