package report

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var metricsCSVHeader = []string{"started_at_utc", "points_in", "points_out", "workers", "duration_ms", "mb_in", "mb_out"}

func AppendMetricsCSV(path string, startedAt time.Time, pointsIn int, pointsOut int, workers int, bytesIn int64, bytesOut int64, elapsed time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	w := csv.NewWriter(f)
	if info.Size() == 0 {
		if err := w.Write(metricsCSVHeader); err != nil {
			return err
		}
	}

	row := []string{
		startedAt.UTC().Format(time.RFC3339),
		strconv.Itoa(pointsIn),
		strconv.Itoa(pointsOut),
		strconv.Itoa(workers),
		strconv.FormatInt(elapsed.Milliseconds(), 10),
		strconv.FormatFloat(float64(bytesIn)/1_000_000, 'f', 6, 64),
		strconv.FormatFloat(float64(bytesOut)/1_000_000, 'f', 6, 64),
	}
	if err := w.Write(row); err != nil {
		return err
	}

	w.Flush()
	return w.Error()
}
