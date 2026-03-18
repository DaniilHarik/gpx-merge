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
	"gpx-merge/internal/pool"
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

	files := make([]pool.File, 0, len(found))
	for _, f := range found {
		files = append(files, pool.File{Index: f.Index, RelPath: f.RelPath, AbsPath: f.AbsPath})
	}

	optOpts := optimize.Options{
		SimplifyMeters: cfg.SimplifyMeters,
		MaxErrorMeters: cfg.MaxErrorMeters,
		MinPoints:      cfg.MinPoints,
	}

	fileProc := processor.NewFileProcessor(cfg, optOpts)
	results, err := pool.Run(ctx, files, cfg.Workers, fileProc.Process)
	if err != nil {
		fmt.Fprintf(stderr, "process files: %v\n", err)
		return 1
	}
	agg := processor.AggregateResults(results, len(found), cfg.Verbose, stdout)
	totals := agg.Totals
	allTracks := agg.AllTracks
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

	bytesOut, writeErr := writeOutput(cfg, allTracks, writeOpts)
	if writeErr != nil {
		fmt.Fprintln(stderr, writeErr)
		return 2
	}
	totals.BytesOut = bytesOut

	elapsed := time.Since(start)
	if elapsed > 0 {
		totals.FilesPerSec = float64(len(results)) / elapsed.Seconds()
		totals.PointsPerSec = float64(totals.PointsIn) / elapsed.Seconds()
	}
	totals.PointReduction = report.ReductionPct(totals.PointsIn, totals.PointsOut)
	totals.ByteReduction = report.ReductionPct64(totals.BytesIn, totals.BytesOut)
	totals.DistanceReduction = report.ReductionPctFloat(totals.DistanceInM, totals.DistanceOutM)

	report.PrintSummary(stdout, totals, elapsed, cfg.Workers)
	report.PrintWarnings(stdout, warningsOut)

	if err := writeReports(cfg, start, elapsed, totals); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	return 0
}

func writeOutput(cfg cli.Config, allTracks []gpx.Track, writeOpts gpx.WriteOptions) (int64, error) {
	if cfg.DryRun {
		bytesOut, err := gpx.MeasureMerged(allTracks, writeOpts)
		if err != nil {
			return 0, fmt.Errorf("measure merged output: %w", err)
		}
		return bytesOut, nil
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0o755); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}
	f, err := os.Create(cfg.Output)
	if err != nil {
		return 0, fmt.Errorf("create output file: %w", err)
	}
	bytesOut, writeErr := gpx.WriteMerged(f, allTracks, writeOpts)
	closeErr := f.Close()
	if writeErr != nil {
		return 0, fmt.Errorf("write output: %w", writeErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("finalize output: %w", closeErr)
	}
	return bytesOut, nil
}

func writeReports(cfg cli.Config, start time.Time, elapsed time.Duration, totals report.Totals) error {
	if cfg.MetricsCSV != "" {
		if err := report.AppendMetricsCSV(cfg.MetricsCSV, report.MetricsRow{
			StartedAt: start,
			PointsIn:  totals.PointsIn,
			PointsOut: totals.PointsOut,
			Workers:   cfg.Workers,
			BytesIn:   totals.BytesIn,
			BytesOut:  totals.BytesOut,
			Elapsed:   elapsed,
		}); err != nil {
			return fmt.Errorf("write --metrics-csv: %w", err)
		}
	}
	return nil
}
