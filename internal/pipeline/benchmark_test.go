package pipeline

import (
	"context"
	"fmt"
	"testing"
)

func benchmarkRun(b *testing.B, workers int) {
	files := make([]File, 1000)
	for i := range files {
		files[i] = File{Index: i, RelPath: fmt.Sprintf("%04d.gpx", i), AbsPath: ""}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Run(context.Background(), files, workers, func(ctx context.Context, f File) (any, error) {
			_ = ctx
			return f.Index, nil
		})
	}
}

func BenchmarkRunWorkers1(b *testing.B) { benchmarkRun(b, 1) }
func BenchmarkRunWorkers4(b *testing.B) { benchmarkRun(b, 4) }
func BenchmarkRunWorkers8(b *testing.B) { benchmarkRun(b, 8) }
