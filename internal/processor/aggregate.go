package processor

import (
	"errors"
	"fmt"
	"io"
	"time"

	"gpx-merge/internal/gpx"
	"gpx-merge/internal/pipeline"
	"gpx-merge/internal/report"
)

type RunAggregation struct {
	Totals      report.Totals
	AllTracks   []gpx.Track
	FileStats   []report.FileStat
	ErrorsOut   []report.ErrorItem
	WarningsOut []report.WarningItem
}

func AggregateResults(results []pipeline.Result, filesScanned int, verbose bool, stdout io.Writer) RunAggregation {
	totals := report.Totals{FilesScanned: filesScanned}
	allTracks := make([]gpx.Track, 0)
	fileStats := make([]report.FileStat, 0, len(results))
	errorsOut := make([]report.ErrorItem, 0)
	warningsOut := make([]report.WarningItem, 0)

	for _, res := range results {
		stat := report.FileStat{
			Index:      res.File.Index,
			Path:       res.File.RelPath,
			DurationMs: res.Duration.Milliseconds(),
		}
		if res.Err != nil {
			stage := "process"
			if ferr := new(fileError); errors.As(res.Err, &ferr) {
				stage = ferr.Stage
			}
			stat.Status = "error"
			stat.Stage = stage
			stat.Error = res.Err.Error()
			totals.FilesFailed++
			errorsOut = append(errorsOut, report.ErrorItem{Path: res.File.RelPath, Stage: stage, Message: res.Err.Error()})
			fileStats = append(fileStats, stat)
			if verbose {
				fmt.Fprintf(stdout, "[error] %s stage=%s err=%v\n", res.File.RelPath, stage, res.Err)
			}
			continue
		}

		payload, ok := res.Payload.(filePayload)
		if !ok {
			stat.Status = "error"
			stat.Stage = "internal"
			stat.Error = "internal payload type mismatch"
			totals.FilesFailed++
			errorsOut = append(errorsOut, report.ErrorItem{Path: res.File.RelPath, Stage: "internal", Message: stat.Error})
			fileStats = append(fileStats, stat)
			continue
		}

		allTracks = append(allTracks, payload.Tracks...)
		totals.FilesProcessed++
		totals.PointsIn += payload.PointsIn
		totals.PointsOut += payload.PointsOut
		totals.BytesIn += payload.BytesIn
		totals.BytesOut += payload.BytesOut
		totals.DistanceInM += payload.DistanceInM
		totals.DistanceOutM += payload.DistanceOutM

		stat.Status = "ok"
		stat.PointsIn = payload.PointsIn
		stat.PointsOut = payload.PointsOut
		stat.BytesIn = payload.BytesIn
		stat.BytesOut = payload.BytesOut
		stat.DistanceInM = payload.DistanceInM
		stat.DistanceOutM = payload.DistanceOutM
		stat.PointReduction = report.ReductionPct(payload.PointsIn, payload.PointsOut)
		stat.ByteReduction = report.ReductionPct64(payload.BytesIn, payload.BytesOut)
		stat.DistanceReduction = report.ReductionPctFloat(payload.DistanceInM, payload.DistanceOutM)
		fileStats = append(fileStats, stat)
		warningsOut = append(warningsOut, payload.Warnings...)
		if verbose {
			fmt.Fprintf(stdout, "[ok] %s points %d->%d (%.2f%%), size %.2fMB->%.2fMB (%.2f%%), distance %.2fkm->%.2fkm (%.2f%%) in %s\n",
				res.File.RelPath,
				payload.PointsIn, payload.PointsOut, stat.PointReduction,
				bytesToMB(payload.BytesIn), bytesToMB(payload.BytesOut), stat.ByteReduction,
				payload.DistanceInM/1000, payload.DistanceOutM/1000, stat.DistanceReduction,
				res.Duration.Round(time.Millisecond),
			)
		}
	}

	return RunAggregation{
		Totals:      totals,
		AllTracks:   allTracks,
		FileStats:   fileStats,
		ErrorsOut:   errorsOut,
		WarningsOut: warningsOut,
	}
}

func MetadataDesc(t report.Totals) string {
	return fmt.Sprintf("files_scanned=%d files_processed=%d files_failed=%d points_in=%d points_out=%d distance_in_km=%.3f distance_out_km=%.3f",
		t.FilesScanned,
		t.FilesProcessed,
		t.FilesFailed,
		t.PointsIn,
		t.PointsOut,
		t.DistanceInM/1000,
		t.DistanceOutM/1000,
	)
}

func bytesToMB(v int64) float64 {
	return float64(v) / 1_000_000
}
