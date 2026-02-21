package pipeline

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

func TestRunDeterministicOrder(t *testing.T) {
	t.Parallel()
	files := []File{
		{Index: 0, RelPath: "a.gpx", AbsPath: "a.gpx"},
		{Index: 1, RelPath: "b.gpx", AbsPath: "b.gpx"},
		{Index: 2, RelPath: "c.gpx", AbsPath: "c.gpx"},
		{Index: 3, RelPath: "d.gpx", AbsPath: "d.gpx"},
	}

	rng := rand.New(rand.NewSource(42))
	results := Run(context.Background(), files, 4, func(ctx context.Context, f File) (any, error) {
		_ = ctx
		time.Sleep(time.Duration(rng.Intn(25)) * time.Millisecond)
		return f.RelPath, nil
	})

	if len(results) != len(files) {
		t.Fatalf("len(results)=%d want %d", len(results), len(files))
	}
	for i, r := range results {
		if r.File.Index != i {
			t.Fatalf("result[%d].File.Index=%d", i, r.File.Index)
		}
	}
}
