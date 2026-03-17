package pool

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

type File struct {
	Index   int
	RelPath string
	AbsPath string
}

type Result struct {
	File     File
	Payload  any
	Duration time.Duration
}

func Run(ctx context.Context, files []File, workers int, process func(context.Context, File) (any, error)) ([]Result, error) {
	if workers < 1 {
		workers = 1
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	collected := make([]Result, len(files))

	for _, f := range files {
		g.Go(func() error {
			start := time.Now()
			payload, err := process(ctx, f)
			if err != nil {
				return err
			}
			collected[f.Index] = Result{File: f, Payload: payload, Duration: time.Since(start)}
			return nil
		})
	}

	return collected, g.Wait()
}
