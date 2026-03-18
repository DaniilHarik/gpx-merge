package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

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

type WarningItem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
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
