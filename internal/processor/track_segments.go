package processor

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gpx-merge/internal/gpx"
	"gpx-merge/internal/report"
)

const segmentDiscontinuityWarnMeters = 1000.0

func fileWarnings(path string, tracks []gpx.Track, splitGapMeters float64) []report.WarningItem {
	largeGapCount := 0
	splitGapCount := 0
	maxGapMeters := 0.0

	for _, trk := range tracks {
		for i := 1; i < len(trk.Segments); i++ {
			prev := trk.Segments[i-1]
			next := trk.Segments[i]
			if len(prev.Points) < 2 || len(next.Points) < 2 {
				continue
			}
			a := prev.Points[len(prev.Points)-1]
			b := next.Points[0]
			gapMeters := haversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
			if gapMeters > maxGapMeters {
				maxGapMeters = gapMeters
			}
			if gapMeters > segmentDiscontinuityWarnMeters {
				largeGapCount++
				if splitGapMeters > 0 && gapMeters > splitGapMeters {
					splitGapCount++
				}
			}
		}
	}

	if largeGapCount == 0 {
		return nil
	}

	var outcome string
	if splitGapMeters <= 0 {
		outcome = "splitting is disabled (--split-track-gap=0), some viewers may draw straight connectors"
	} else if splitGapCount == 0 {
		outcome = fmt.Sprintf("none exceed split threshold %.0fm, some viewers may draw straight connectors", splitGapMeters)
	} else if splitGapCount == largeGapCount {
		outcome = fmt.Sprintf("all exceed split threshold %.0fm and will be separated into track parts", splitGapMeters)
	} else {
		outcome = fmt.Sprintf("%d/%d exceed split threshold %.0fm and will be separated into track parts", splitGapCount, largeGapCount, splitGapMeters)
	}

	msg := fmt.Sprintf("segment discontinuities: %d gap(s) > %.0fm, max %.1fm; %s",
		largeGapCount,
		segmentDiscontinuityWarnMeters,
		maxGapMeters,
		outcome,
	)
	return []report.WarningItem{{Path: path, Message: msg}}
}

func sortTrackSegmentsByFirstTimestamp(tracks []gpx.Track) ([]gpx.Track, int) {
	if len(tracks) == 0 {
		return tracks, 0
	}
	out := make([]gpx.Track, len(tracks))
	reorderedCount := 0
	for i, trk := range tracks {
		sortedTrack, reordered := sortSingleTrackSegmentsByFirstTimestamp(trk)
		out[i] = sortedTrack
		if reordered {
			reorderedCount++
		}
	}
	return out, reorderedCount
}

func sortSingleTrackSegmentsByFirstTimestamp(track gpx.Track) (gpx.Track, bool) {
	if len(track.Segments) < 2 {
		return track, false
	}

	type segOrder struct {
		seg   gpx.Segment
		first time.Time
		idx   int
	}

	ordered := make([]segOrder, len(track.Segments))
	for i, seg := range track.Segments {
		ts, ok := segmentFirstTimestamp(seg)
		if !ok {
			return track, false
		}
		ordered[i] = segOrder{seg: seg, first: ts, idx: i}
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].first.Equal(ordered[j].first) {
			return ordered[i].idx < ordered[j].idx
		}
		return ordered[i].first.Before(ordered[j].first)
	})

	reordered := false
	sortedSegs := make([]gpx.Segment, len(ordered))
	for i, item := range ordered {
		sortedSegs[i] = item.seg
		if item.idx != i {
			reordered = true
		}
	}
	if !reordered {
		return track, false
	}

	return gpx.Track{
		Name:     track.Name,
		Segments: sortedSegs,
	}, true
}

func segmentFirstTimestamp(seg gpx.Segment) (time.Time, bool) {
	for _, p := range seg.Points {
		raw := strings.TrimSpace(p.Time)
		if raw == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, raw)
		if err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}
