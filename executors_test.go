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
	executors := rxp.New()
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
	t.Log("done", executors.Goroutines())
}

func TestExecutors_CloseTimeout(t *testing.T) {
	tasks := 100
	counter := new(atomic.Int64)
	executors := rxp.New(rxp.WithCloseTimeout(time.Second))
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
	executors := rxp.New()
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

	err := executors.Close()
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
// BenchmarkExecutors_TryExecute-20    	 2680070	       426.4 ns/op	         0 failed	       0 B/op	       0 allocs/op
func BenchmarkExecutors_TryExecute(b *testing.B) {
	b.ReportAllocs()
	executors := rxp.New()
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
	err := executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkTryExecute
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkTryExecute
// BenchmarkTryExecute-20    	 2940900	       385.0 ns/op	         0 failed	       0 B/op	       0 allocs/op
func BenchmarkTryExecute(b *testing.B) {
	b.ReportAllocs()
	executors := rxp.New()
	ctx := executors.Context()
	failed := new(atomic.Int64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok := executors.TryExecute(ctx, RandTask)
		if !ok {
			failed.Add(1)
		}
	}
	err := executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkExecutors_UnlimitedExecute
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkExecutors_UnlimitedExecute
// BenchmarkExecutors_UnlimitedExecute-20    	 6444103	       186.6 ns/op	         0 failed	      34 B/op	       1 allocs/op
func BenchmarkExecutors_UnlimitedExecute(b *testing.B) {
	b.ReportAllocs()
	executors := rxp.New()
	ctx := executors.Context()
	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := executors.UnlimitedExecute(ctx, RandTask)
			if err != nil {
				failed.Add(1)
			}
		}
	})
	err := executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkExecutors_UnlimitedExecute
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkExecutors_DirectExecute
// BenchmarkExecutors_DirectExecute-20    	 6159405	       192.1 ns/op	         0 failed	      38 B/op	       1 allocs/op
func BenchmarkExecutors_DirectExecute(b *testing.B) {
	b.ReportAllocs()
	executors := rxp.New()
	ctx := executors.Context()
	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := executors.DirectExecute(ctx, RandTask)
			if err != nil {
				failed.Add(1)
			}
		}
	})
	err := executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
