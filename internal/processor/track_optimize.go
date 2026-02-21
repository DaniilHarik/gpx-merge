package processor

import (
	"fmt"
	"math"

	"gpx-merge/internal/gpx"
	"gpx-merge/internal/optimize"
)

func optimizeTrack(track gpx.Track, opts optimize.Options, keepEle, keepTime bool) (gpx.Track, int, int, float64, float64, error) {
	out := gpx.Track{Name: track.Name, Segments: make([]gpx.Segment, 0, len(track.Segments))}
	pointsIn := 0
	pointsOut := 0
	distanceInM := 0.0
	distanceOutM := 0.0

	for _, seg := range track.Segments {
		if len(seg.Points) < 2 {
			return gpx.Track{}, 0, 0, 0, 0, fmt.Errorf("segment has %d points; expected at least 2", len(seg.Points))
		}
		coords := make([]optimize.Coord, len(seg.Points))
		for i, p := range seg.Points {
			coords[i] = optimize.Coord{Lat: p.Lat, Lon: p.Lon}
		}

		distanceInM += segmentDistanceMeters(seg.Points)

		res := optimize.SimplifyIndices(coords, opts)
		selected := make([]gpx.Point, 0, len(res.Indexes))
		for _, idx := range res.Indexes {
			pt := seg.Points[idx]
			if !keepEle {
				pt.Ele = nil
			}
			if !keepTime {
				pt.Time = ""
			}
			selected = append(selected, pt)
		}
		if len(selected) < 2 {
			return gpx.Track{}, 0, 0, 0, 0, fmt.Errorf("optimized segment became invalid (%d points)", len(selected))
		}
		distanceOutM += segmentDistanceMeters(selected)

		out.Segments = append(out.Segments, gpx.Segment{Name: seg.Name, Points: selected})
		pointsIn += len(seg.Points)
		pointsOut += len(selected)
	}

	return out, pointsIn, pointsOut, distanceInM, distanceOutM, nil
}

func segmentDistanceMeters(points []gpx.Point) float64 {
	if len(points) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += haversineMeters(points[i-1].Lat, points[i-1].Lon, points[i].Lat, points[i].Lon)
	}
	return total
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMeters = 6371000.0
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	a := sinLat*sinLat + math.Cos(lat1Rad)*math.Cos(lat2Rad)*sinLon*sinLon
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}
