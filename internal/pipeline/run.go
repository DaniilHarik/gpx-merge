package pipeline

import (
	"context"
	"sort"
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

	jobs := make(chan File)
	results := make(chan Result, workers*2)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				start := time.Now()
				payload, err := process(ctx, f)
				res := Result{File: f, Payload: payload, Err: err, Duration: time.Since(start)}
				select {
				case results <- res:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, f := range files {
			select {
			case jobs <- f:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	collected := make([]Result, 0, len(files))
	for res := range results {
		collected = append(collected, res)
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].File.Index < collected[j].File.Index
	})
	return collected
}
