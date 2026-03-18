package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultInput     = "./data"
	defaultOutput    = "./out/merged_optimized.gpx"
	defaultWorkers   = 16
	defaultSimplify  = 0.8
	defaultMaxError  = 1.5
	defaultSplitGap  = 1000.0
	defaultPrecision = 6
	defaultMinPoints = 2
)

type Config struct {
	Input               string
	Output              string
	Workers             int
	SimplifyMeters      float64
	MaxErrorMeters      float64
	SplitTrackGapMeters float64
	SortSegmentsByTime  bool
	Precision           int
	MinPoints           int
	KeepTime            bool
	KeepEle             bool
	DryRun              bool
	Verbose             bool
	IncludeRunMetadata  bool
	MetricsCSV          string
}

type UsageError struct {
	Message string
	Usage   string
}

func (e *UsageError) Error() string {
	return e.Message
}

func (e *UsageError) FullMessage() string {
	if e.Usage == "" {
		return e.Message
	}
	return fmt.Sprintf("%s\n\n%s", e.Message, e.Usage)
}

func Parse(args []string) (Config, error) {
	cfg := Config{}

	var buf bytes.Buffer
	fs := flag.NewFlagSet("gpx-merge", flag.ContinueOnError)
	fs.SetOutput(&buf)

	fs.StringVar(&cfg.Input, "input", defaultInput, "root folder containing GPX files")
	fs.StringVar(&cfg.Output, "output", defaultOutput, "merged GPX output path")
	fs.IntVar(&cfg.Workers, "workers", defaultWorkers, "worker pool size")
	fs.Float64Var(&cfg.SimplifyMeters, "simplify", defaultSimplify, "base simplification tolerance in meters")
	fs.Float64Var(&cfg.MaxErrorMeters, "max-error", defaultMaxError, "hard cap for allowed geometric deviation")
	fs.Float64Var(&cfg.SplitTrackGapMeters, "split-track-gap", defaultSplitGap, "split a track into multiple <trk> when segment endpoint gaps exceed this many meters (0 disables)")
	fs.BoolVar(&cfg.SortSegmentsByTime, "sort-segments-by-time", false, "reorder track segments by first timestamp before optimization")
	fs.IntVar(&cfg.Precision, "precision", defaultPrecision, "coordinate decimal precision")
	fs.IntVar(&cfg.MinPoints, "min-points", defaultMinPoints, "minimum points to keep per segment")
	fs.BoolVar(&cfg.KeepTime, "keep-time", false, "preserve <time> tags")
	fs.BoolVar(&cfg.KeepEle, "keep-ele", false, "preserve <ele> tags")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "report projected savings without writing output")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "per-file optimization stats")
	fs.BoolVar(&cfg.IncludeRunMetadata, "include-run-metadata", false, "include generation stats in metadata")
	fs.StringVar(&cfg.MetricsCSV, "metrics-csv", "", "append run metrics to a CSV file")

	if err := fs.Parse(args); err != nil {
		return Config{}, &UsageError{Message: err.Error(), Usage: usageText(fs, &buf)}
	}
	if fs.NArg() > 0 {
		return Config{}, &UsageError{Message: fmt.Sprintf("unexpected positional arguments: %v", fs.Args()), Usage: usageText(fs, &buf)}
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func usageText(fs *flag.FlagSet, parseOut io.Reader) string {
	b, _ := io.ReadAll(parseOut)
	if len(b) > 0 {
		return string(b)
	}
	var buf bytes.Buffer
	buf.WriteString("Usage: gpx-merge [flags]\n")
	fs.SetOutput(&buf)
	fs.PrintDefaults()
	return buf.String()
}

func validate(cfg Config) error {
	if cfg.Workers <= 0 {
		return errors.New("--workers must be > 0")
	}
	if cfg.SimplifyMeters <= 0 {
		return errors.New("--simplify must be > 0")
	}
	if cfg.MaxErrorMeters <= 0 {
		return errors.New("--max-error must be > 0")
	}
	if cfg.SplitTrackGapMeters < 0 {
		return errors.New("--split-track-gap must be >= 0")
	}
	if cfg.Precision < 0 || cfg.Precision > 15 {
		return errors.New("--precision must be between 0 and 15")
	}
	if cfg.MinPoints < 2 {
		return errors.New("--min-points must be >= 2")
	}
	if cfg.Input == "" {
		return errors.New("--input cannot be empty")
	}
	info, err := os.Stat(cfg.Input)
	if err != nil {
		return fmt.Errorf("invalid --input: %w", err)
	}
	if !info.IsDir() {
		return errors.New("--input must be a directory")
	}
	absInput, err := filepath.Abs(cfg.Input)
	if err != nil {
		return fmt.Errorf("resolve --input: %w", err)
	}
	if cfg.Output == "" && !cfg.DryRun {
		return errors.New("--output cannot be empty")
	}
	if cfg.Output != "" {
		if dir := filepath.Dir(cfg.Output); dir == "" {
			return errors.New("invalid --output path")
		}
		if !cfg.DryRun {
			absOutput, err := filepath.Abs(cfg.Output)
			if err != nil {
				return fmt.Errorf("resolve --output: %w", err)
			}
			if pathWithinDir(absOutput, absInput) {
				return errors.New("--output must not be inside --input")
			}
		}
	}
	if cfg.MetricsCSV != "" {
		if dir := filepath.Dir(cfg.MetricsCSV); dir == "" {
			return errors.New("invalid --metrics-csv path")
		}
	}
	return nil
}

func pathWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
