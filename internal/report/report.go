package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ConfigSnapshot struct {
	Input               string  `json:"input"`
	Output              string  `json:"output"`
	Workers             int     `json:"workers"`
	SimplifyMeters      float64 `json:"simplify"`
	MaxErrorMeters      float64 `json:"max_error"`
	SplitTrackGapMeters float64 `json:"split_track_gap_meters"`
	SortSegmentsByTime  bool    `json:"sort_segments_by_time"`
	Precision           int     `json:"precision"`
	MinPoints           int     `json:"min_points"`
	KeepTime            bool    `json:"keep_time"`
	KeepEle             bool    `json:"keep_ele"`
	DryRun              bool    `json:"dry_run"`
	Verbose             bool    `json:"verbose"`
	IncludeRunMetadata  bool    `json:"include_run_metadata"`
	JSONReport          string  `json:"json_report,omitempty"`
	MetricsCSV          string  `json:"metrics_csv,omitempty"`
}

type Totals struct {
	FilesScanned      int     `json:"files_scanned"`
	FilesProcessed    int     `json:"files_processed"`
	FilesFailed       int     `json:"files_failed"`
	PointsIn          int     `json:"points_in"`
	PointsOut         int     `json:"points_out"`
	BytesIn           int64   `json:"bytes_in"`
	BytesOut          int64   `json:"bytes_out"`
	DistanceInM       float64 `json:"distance_in_m"`
	DistanceOutM      float64 `json:"distance_out_m"`
	PointReduction    float64 `json:"point_reduction_pct"`
	ByteReduction     float64 `json:"byte_reduction_pct"`
	DistanceReduction float64 `json:"distance_reduction_pct"`
	FilesPerSec       float64 `json:"files_per_sec"`
	PointsPerSec      float64 `json:"points_per_sec"`
}

type FileStat struct {
	Index             int     `json:"index"`
	Path              string  `json:"path"`
	Status            string  `json:"status"`
	Stage             string  `json:"stage,omitempty"`
	Error             string  `json:"error,omitempty"`
	PointsIn          int     `json:"points_in"`
	PointsOut         int     `json:"points_out"`
	BytesIn           int64   `json:"bytes_in"`
	BytesOut          int64   `json:"bytes_out"`
	DistanceInM       float64 `json:"distance_in_m"`
	DistanceOutM      float64 `json:"distance_out_m"`
	PointReduction    float64 `json:"point_reduction_pct"`
	ByteReduction     float64 `json:"byte_reduction_pct"`
	DistanceReduction float64 `json:"distance_reduction_pct"`
	DurationMs        int64   `json:"duration_ms"`
}

type ErrorItem struct {
	Path    string `json:"path"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type WarningItem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type JSONReport struct {
	StartedAt  string         `json:"started_at"`
	FinishedAt string         `json:"finished_at"`
	DurationMs int64          `json:"duration_ms"`
	Config     ConfigSnapshot `json:"config"`
	Totals     Totals         `json:"totals"`
	Files      []FileStat     `json:"files"`
	Errors     []ErrorItem    `json:"errors"`
	Warnings   []WarningItem  `json:"warnings,omitempty"`
}

func ReductionPct(in, out int) float64 {
	if in <= 0 {
		return 0
	}
	return float64(in-out) / float64(in) * 100
}

func ReductionPct64(in, out int64) float64 {
	if in <= 0 {
		return 0
	}
	return float64(in-out) / float64(in) * 100
}

func ReductionPctFloat(in, out float64) float64 {
	if in <= 0 {
		return 0
	}
	return (in - out) / in * 100
}

func WriteJSON(path string, report JSONReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func PrintSummary(w io.Writer, totals Totals, elapsed time.Duration, workers int) {
	fmt.Fprintf(w, "Files scanned: %s\n", formatGroupedInt(totals.FilesScanned))
	fmt.Fprintf(w, "Files processed: %s\n", formatGroupedInt(totals.FilesProcessed))
	fmt.Fprintf(w, "Files failed: %s\n", formatGroupedInt(totals.FilesFailed))
	fmt.Fprintf(w, "Workers: %s\n", formatGroupedInt(workers))
	fmt.Fprintf(w, "Points: %s -> %s (%.2f%% reduction)\n", formatGroupedInt(totals.PointsIn), formatGroupedInt(totals.PointsOut), totals.PointReduction)
	fmt.Fprintf(w, "Size: %s -> %s (%.2f%% reduction)\n", formatMB(totals.BytesIn), formatMB(totals.BytesOut), totals.ByteReduction)
	fmt.Fprintf(w, "Distance: %.2f km -> %.2f km (%.2f%% reduction)\n", totals.DistanceInM/1000, totals.DistanceOutM/1000, totals.DistanceReduction)
	fmt.Fprintf(w, "Elapsed: %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(w, "Throughput: %.2f files/s, %.2f points/s\n", totals.FilesPerSec, totals.PointsPerSec)
}

func PrintFailedFiles(w io.Writer, errs []ErrorItem) {
	if len(errs) == 0 {
		return
	}
	fmt.Fprintln(w, "Failed files:")
	for _, e := range errs {
		reason := strings.TrimPrefix(e.Message, e.Stage+": ")
		fmt.Fprintf(w, "- %s (%s): %s\n", e.Path, e.Stage, reason)
	}
}

func PrintWarnings(w io.Writer, warnings []WarningItem) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "Warnings:")
	for _, wi := range warnings {
		fmt.Fprintf(w, "- %s: %s\n", wi.Path, wi.Message)
	}
}

func formatGroupedInt(v int) string {
	return formatGroupedInt64(int64(v))
}

func formatMB(v int64) string {
	return fmt.Sprintf("%.2f MB", float64(v)/1_000_000)
}

func formatGroupedInt64(v int64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	s := strconv.FormatInt(v, 10)
	if len(s) <= 3 {
		return sign + s
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(ch)
	}
	return sign + b.String()
}
