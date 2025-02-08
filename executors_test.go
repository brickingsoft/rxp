package rxp_test

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

func RandTask(_ context.Context) {
	rand.Float64()
}

func TestExecutors_TryExecute(t *testing.T) {
	tasks := 100
	counter := new(atomic.Int64)
	executors, err := rxp.New()
	if err != nil {
		t.Error(err)
		return
	}
	ctx := executors.Context()
	defer func(executors rxp.Executors) {
		err := executors.Close()
		if err != nil {
			t.Fatal(err)
			return
		}
		t.Log("goroutines", executors.Goroutines())
		t.Log("executed", counter.Load(), counter.Load() == int64(tasks))
	}(executors)
	submitted := 0
	for i := 0; i < tasks; i++ {
		if executors.TryExecute(ctx, func(ctx context.Context) {
			counter.Add(1)
		}) {
			submitted++
		}
	}
	t.Log("submitted", submitted)
	t.Log("goroutines", executors.Goroutines())
}

func TestExecutors_CloseTimeout(t *testing.T) {
	tasks := 100
	counter := new(atomic.Int64)
	executors, err := rxp.New(rxp.WithCloseTimeout(time.Second))
	if err != nil {
		t.Error(err)
		return
	}
	ctx := executors.Context()
	defer func(executors rxp.Executors) {
		err := executors.Close()
		if err != nil {
			t.Log("executed", counter.Load(), counter.Load() == int64(tasks))
			if errors.Is(err, context.DeadlineExceeded) {
				t.Log(err)
				return
			}
			t.Fatal(err)
			return
		}
		t.Log("goroutines", executors.Goroutines())
		t.Log("executed", counter.Load(), counter.Load() == int64(tasks))
	}(executors)
	submitted := 0
	for i := 0; i < tasks; i++ {
		if executors.TryExecute(ctx, func(ctx context.Context) {
			time.Sleep(time.Second * 2)
			select {
			case <-ctx.Done():
				break
			default:
				counter.Add(1)
			}
		}) {
			submitted++
		}
	}
	t.Log("submitted", submitted)
	t.Log("goroutines", executors.Goroutines())
	t.Log("done", executors.Goroutines())
}

// BenchmarkExecutors_Execute
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkExecutors_Execute
// BenchmarkExecutors_Execute-20    	 2636208	       429.0 ns/op	         0 failed	       0 B/op	       0 allocs/op
func BenchmarkExecutors_Execute(b *testing.B) {
	b.ReportAllocs()
	executors, err := rxp.New()
	if err != nil {
		b.Error(err)
		return
	}
	ctx := executors.Context()
	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := executors.Execute(ctx, RandTask)
			if err != nil {
				failed.Add(1)
			}
		}
	})

	err = executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkExecutors_TryExecute_Parallel
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkExecutors_TryExecute_Parallel
// BenchmarkExecutors_TryExecute_Parallel-20    	 2670756	       412.1 ns/op	         0 failed	       0 B/op	       0 allocs/op
func BenchmarkExecutors_TryExecute_Parallel(b *testing.B) {
	b.ReportAllocs()
	executors, err := rxp.New()
	if err != nil {
		b.Error(err)
		return
	}
	ctx := executors.Context()
	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ok := executors.TryExecute(ctx, RandTask)
			if !ok {
				failed.Add(1)
			}
		}
	})
	err = executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkExecutors_TryExecute
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkExecutors_TryExecute
// BenchmarkExecutors_TryExecute-20    	 2940900	       385.0 ns/op	         0 failed	       0 B/op	       0 allocs/op
func BenchmarkExecutors_TryExecute(b *testing.B) {
	b.ReportAllocs()
	executors, err := rxp.New()
	if err != nil {
		b.Error(err)
		return
	}
	ctx := executors.Context()
	failed := new(atomic.Int64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok := executors.TryExecute(ctx, RandTask)
		if !ok {
			failed.Add(1)
		}
	}
	err = executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
