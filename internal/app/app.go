package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gpx-merge/internal/cli"
	"gpx-merge/internal/discovery"
	"gpx-merge/internal/gpx"
	"gpx-merge/internal/optimize"
	"gpx-merge/internal/pipeline"
	"gpx-merge/internal/processor"
	"gpx-merge/internal/report"
)

var Version = "dev"

func Main() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	start := time.Now()

	cfg, err := cli.Parse(args)
	if err != nil {
		var ue *cli.UsageError
		if errors.As(err, &ue) {
			fmt.Fprintln(stderr, ue.FullMessage())
			return 2
		}
		fmt.Fprintf(stderr, "configuration error: %v\n", err)
		return 2
	}

	found, err := discovery.Discover(cfg.Input)
	if err != nil {
		fmt.Fprintf(stderr, "discover input files: %v\n", err)
		return 2
	}
	if len(found) == 0 {
		fmt.Fprintln(stderr, "configuration error: no .gpx files found under --input")
		return 2
	}

	files := make([]pipeline.File, 0, len(found))
	for _, f := range found {
		files = append(files, pipeline.File{Index: f.Index, RelPath: f.RelPath, AbsPath: f.AbsPath})
	}

	optOpts := optimize.Options{
		SimplifyMeters: cfg.SimplifyMeters,
		MaxErrorMeters: cfg.MaxErrorMeters,
		MinPoints:      cfg.MinPoints,
	}

	fileProc := processor.NewFileProcessor(cfg, optOpts)
	results := pipeline.Run(ctx, files, cfg.Workers, fileProc.Process)
	agg := processor.AggregateResults(results, len(found), cfg.Verbose, stdout)
	totals := agg.Totals
	allTracks := agg.AllTracks
	fileStats := agg.FileStats
	errorsOut := agg.ErrorsOut
	warningsOut := agg.WarningsOut

	writeOpts := gpx.WriteOptions{
		Creator:             fmt.Sprintf("gpx-merge/%s", Version),
		Precision:           cfg.Precision,
		KeepTime:            cfg.KeepTime,
		KeepEle:             cfg.KeepEle,
		SplitTrackGapMeters: cfg.SplitTrackGapMeters,
		IncludeMetadata:     cfg.IncludeRunMetadata,
		MetadataTime:        time.Now().UTC(),
	}
	writeOpts.MetadataDesc = processor.MetadataDesc(totals)

	if cfg.DryRun {
		bytesOut, measureErr := gpx.MeasureMerged(allTracks, writeOpts)
		if measureErr != nil {
			fmt.Fprintf(stderr, "measure merged output: %v\n", measureErr)
			return 2
		}
		totals.BytesOut = bytesOut
	} else {
		if err := os.MkdirAll(filepath.Dir(cfg.Output), 0o755); err != nil {
			fmt.Fprintf(stderr, "create output directory: %v\n", err)
			return 2
		}
		f, createErr := os.Create(cfg.Output)
		if createErr != nil {
			fmt.Fprintf(stderr, "create output file: %v\n", createErr)
			return 2
		}
		bytesOut, writeErr := gpx.WriteMerged(f, allTracks, writeOpts)
		closeErr := f.Close()
		if writeErr != nil {
			fmt.Fprintf(stderr, "write output: %v\n", writeErr)
			return 2
		}
		if closeErr != nil {
			fmt.Fprintf(stderr, "finalize output: %v\n", closeErr)
			return 2
		}
		totals.BytesOut = bytesOut
	}

	elapsed := time.Since(start)
	if elapsed > 0 {
		totals.FilesPerSec = float64(len(results)) / elapsed.Seconds()
		totals.PointsPerSec = float64(totals.PointsIn) / elapsed.Seconds()
	}
	totals.PointReduction = report.ReductionPct(totals.PointsIn, totals.PointsOut)
	totals.ByteReduction = report.ReductionPct64(totals.BytesIn, totals.BytesOut)
	totals.DistanceReduction = report.ReductionPctFloat(totals.DistanceInM, totals.DistanceOutM)

	report.PrintSummary(stdout, totals, elapsed, cfg.Workers)
	report.PrintFailedFiles(stdout, errorsOut)
	report.PrintWarnings(stdout, warningsOut)

	if cfg.JSONReport != "" {
		jr := report.JSONReport{
			StartedAt:  start.UTC().Format(time.RFC3339),
			FinishedAt: time.Now().UTC().Format(time.RFC3339),
			DurationMs: elapsed.Milliseconds(),
			Config: report.ConfigSnapshot{
				Input:               cfg.Input,
				Output:              cfg.Output,
				Workers:             cfg.Workers,
				SimplifyMeters:      cfg.SimplifyMeters,
				MaxErrorMeters:      cfg.MaxErrorMeters,
				SplitTrackGapMeters: cfg.SplitTrackGapMeters,
				SortSegmentsByTime:  cfg.SortSegmentsByTime,
				Precision:           cfg.Precision,
				MinPoints:           cfg.MinPoints,
				KeepTime:            cfg.KeepTime,
				KeepEle:             cfg.KeepEle,
				DryRun:              cfg.DryRun,
				Verbose:             cfg.Verbose,
				IncludeRunMetadata:  cfg.IncludeRunMetadata,
				JSONReport:          cfg.JSONReport,
			},
			Totals:   totals,
			Files:    fileStats,
			Errors:   errorsOut,
			Warnings: warningsOut,
		}
		if err := report.WriteJSON(cfg.JSONReport, jr); err != nil {
			fmt.Fprintf(stderr, "write --json-report: %v\n", err)
			return 2
		}
	}

	if totals.FilesFailed > 0 {
		return 1
	}
	return 0
}
