package optimize

import (
	"math"
	"sort"
)

const earthRadiusMeters = 6371000.0

type Coord struct {
	Lat float64
	Lon float64
}

type Options struct {
	SimplifyMeters float64
	MaxErrorMeters float64
	MinPoints      int
}

type Result struct {
	Indexes          []int
	AppliedTolerance float64
	MaxDeviation     float64
}

func SimplifyIndices(points []Coord, opts Options) Result {
	n := len(points)
	if n == 0 {
		return Result{}
	}
	if n == 1 {
		return Result{Indexes: []int{0}}
	}
	if n <= opts.MinPoints {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return Result{Indexes: idx}
	}

	eps := opts.SimplifyMeters
	if eps < 0 {
		eps = 0
	}

	best := simplifyWithEpsilon(points, eps)
	best.AppliedTolerance = eps

	if opts.MaxErrorMeters > 0 && best.MaxDeviation > opts.MaxErrorMeters {
		lo := 0.0
		hi := eps
		fallback := simplifyWithEpsilon(points, 0)
		fallback.AppliedTolerance = 0
		best = fallback

		for range 24 {
			mid := (lo + hi) / 2
			candidate := simplifyWithEpsilon(points, mid)
			candidate.AppliedTolerance = mid
			if candidate.MaxDeviation <= opts.MaxErrorMeters {
				best = candidate
				lo = mid
			} else {
				hi = mid
			}
		}
	}

	best.Indexes = enforceMinPoints(best.Indexes, n, opts.MinPoints)
	best.MaxDeviation = polylineMaxDeviation(points, best.Indexes)
	return best
}

func simplifyWithEpsilon(points []Coord, epsilon float64) Result {
	n := len(points)
	keep := make([]bool, n)
	keep[0], keep[n-1] = true, true

	projected := project(points)
	dp(projected, 0, n-1, epsilon, keep)

	idx := make([]int, 0, n)
	for i, k := range keep {
		if k {
			idx = append(idx, i)
		}
	}
	return Result{Indexes: idx, MaxDeviation: polylineMaxDeviation(points, idx)}
}

func project(points []Coord) [][2]float64 {
	sum := 0.0
	for _, p := range points {
		sum += p.Lat
	}
	refLatRad := radians(sum / float64(len(points)))
	cosRef := math.Cos(refLatRad)

	out := make([][2]float64, len(points))
	for i, p := range points {
		latRad := radians(p.Lat)
		lonRad := radians(p.Lon)
		x := earthRadiusMeters * lonRad * cosRef
		y := earthRadiusMeters * latRad
		out[i] = [2]float64{x, y}
	}
	return out
}

func dp(points [][2]float64, start, end int, epsilon float64, keep []bool) {
	if end-start <= 1 {
		return
	}
	maxDist := -1.0
	maxIdx := -1
	ax, ay := points[start][0], points[start][1]
	bx, by := points[end][0], points[end][1]

	for i := start + 1; i < end; i++ {
		d := pointToSegmentDistance(points[i][0], points[i][1], ax, ay, bx, by)
		if d > maxDist {
			maxDist = d
			maxIdx = i
		}
	}

	if maxIdx >= 0 && maxDist > epsilon {
		keep[maxIdx] = true
		dp(points, start, maxIdx, epsilon, keep)
		dp(points, maxIdx, end, epsilon, keep)
	}
}

func pointToSegmentDistance(px, py, ax, ay, bx, by float64) float64 {
	dx := bx - ax
	dy := by - ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	x := ax + t*dx
	y := ay + t*dy
	return math.Hypot(px-x, py-y)
}

func polylineMaxDeviation(points []Coord, indexes []int) float64 {
	if len(points) <= 2 || len(indexes) <= 1 {
		return 0
	}
	proj := project(points)
	maxDist := 0.0
	for i := 0; i < len(indexes)-1; i++ {
		start := indexes[i]
		end := indexes[i+1]
		ax, ay := proj[start][0], proj[start][1]
		bx, by := proj[end][0], proj[end][1]
		for j := start; j <= end; j++ {
			d := pointToSegmentDistance(proj[j][0], proj[j][1], ax, ay, bx, by)
			if d > maxDist {
				maxDist = d
			}
		}
	}
	return maxDist
}

func enforceMinPoints(indexes []int, total int, minPoints int) []int {
	if minPoints <= 0 || len(indexes) >= minPoints || total == 0 {
		return indexes
	}

	set := make(map[int]struct{}, minPoints)
	for _, idx := range indexes {
		set[idx] = struct{}{}
	}

	if minPoints == 1 {
		return []int{0}
	}
	for i := 0; i < minPoints; i++ {
		idx := int(math.Round(float64(i) * float64(total-1) / float64(minPoints-1)))
		set[idx] = struct{}{}
	}
	for i := 0; i < total && len(set) < minPoints; i++ {
		set[i] = struct{}{}
	}

	out := make([]int, 0, len(set))
	for idx := range set {
		out = append(out, idx)
	}
	sort.Ints(out)
	return out
}

func radians(v float64) float64 {
	return v * math.Pi / 180
}
