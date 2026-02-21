package processor

import (
	"context"
	"fmt"
	"os"

	"gpx-merge/internal/cli"
	"gpx-merge/internal/gpx"
	"gpx-merge/internal/optimize"
	"gpx-merge/internal/pipeline"
	"gpx-merge/internal/report"
)

type filePayload struct {
	Tracks       []gpx.Track
	PointsIn     int
	PointsOut    int
	BytesIn      int64
	BytesOut     int64
	DistanceInM  float64
	DistanceOutM float64
	Warnings     []report.WarningItem
}

type fileError struct {
	Stage string
	Path  string
	Err   error
}

type FileProcessor struct {
	cfg     cli.Config
	optOpts optimize.Options
}

func NewFileProcessor(cfg cli.Config, optOpts optimize.Options) FileProcessor {
	return FileProcessor{
		cfg:     cfg,
		optOpts: optOpts,
	}
}

func (e *fileError) Error() string {
	return fmt.Sprintf("%s: %v", e.Stage, e.Err)
}

func (e *fileError) Unwrap() error {
	return e.Err
}

func (p FileProcessor) Process(ctx context.Context, f pipeline.File) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, &fileError{Stage: "canceled", Path: f.RelPath, Err: err}
	}
	info, err := os.Stat(f.AbsPath)
	if err != nil {
		return nil, &fileError{Stage: "stat", Path: f.RelPath, Err: err}
	}
	tracks, err := gpx.ParseFile(f.AbsPath, f.RelPath)
	if err != nil {
		return nil, &fileError{Stage: "parse", Path: f.RelPath, Err: err}
	}
	warnings := make([]report.WarningItem, 0, 2)
	if p.cfg.SortSegmentsByTime {
		reordered := 0
		tracks, reordered = sortTrackSegmentsByFirstTimestamp(tracks)
		if reordered > 0 {
			warnings = append(warnings, report.WarningItem{
				Path:    f.RelPath,
				Message: fmt.Sprintf("reordered segments by first timestamp in %d track(s) (--sort-segments-by-time)", reordered),
			})
		}
	}

	optimizedTracks := make([]gpx.Track, 0, len(tracks))
	pointsIn := 0
	pointsOut := 0
	distanceInM := 0.0
	distanceOutM := 0.0
	for _, trk := range tracks {
		optTrack, inN, outN, inDistM, outDistM, optErr := optimizeTrack(trk, p.optOpts, p.cfg.KeepEle, p.cfg.KeepTime)
		if optErr != nil {
			return nil, &fileError{Stage: "optimize", Path: f.RelPath, Err: optErr}
		}
		if len(optTrack.Segments) == 0 {
			continue
		}
		optimizedTracks = append(optimizedTracks, optTrack)
		pointsIn += inN
		pointsOut += outN
		distanceInM += inDistM
		distanceOutM += outDistM
	}
	if len(optimizedTracks) == 0 {
		return nil, &fileError{Stage: "optimize", Path: f.RelPath, Err: fmt.Errorf("no valid tracks after optimization")}
	}

	bytesOut, err := gpx.MeasureTracks(optimizedTracks, gpx.WriteOptions{
		Creator:             "",
		Precision:           p.cfg.Precision,
		KeepEle:             p.cfg.KeepEle,
		KeepTime:            p.cfg.KeepTime,
		SplitTrackGapMeters: p.cfg.SplitTrackGapMeters,
	})
	if err != nil {
		return nil, &fileError{Stage: "measure", Path: f.RelPath, Err: err}
	}

	return filePayload{
		Tracks:       optimizedTracks,
		PointsIn:     pointsIn,
		PointsOut:    pointsOut,
		BytesIn:      info.Size(),
		BytesOut:     bytesOut,
		DistanceInM:  distanceInM,
		DistanceOutM: distanceOutM,
		Warnings:     append(warnings, fileWarnings(f.RelPath, optimizedTracks, p.cfg.SplitTrackGapMeters)...),
	}, nil
}
