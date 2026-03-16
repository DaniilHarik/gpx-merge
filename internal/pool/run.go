package pool

import (
	"context"
	"sync"
	"time"
)

type File struct {
	Index   int
	RelPath string
	AbsPath string
}

type Result struct {
	File     File
	Payload  any
	Err      error
	Duration time.Duration
}

func Run(ctx context.Context, files []File, workers int, process func(context.Context, File) (any, error)) []Result {
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan File, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	collected := make([]Result, len(files))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				start := time.Now()
				payload, err := process(ctx, f)
				collected[f.Index] = Result{File: f, Payload: payload, Err: err, Duration: time.Since(start)}
			}
		}()
	}

	wg.Wait()
	return collected
}
