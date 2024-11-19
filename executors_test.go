package rxp_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"math/rand"
	"sync/atomic"
	"testing"
)

func RandTask() {
	rand.Float64()
}

func TestExecutors_TryExecute(t *testing.T) {
	ctx := context.Background()
	executors := rxp.New()
	defer func(executors rxp.Executors) {
		err := executors.CloseGracefully()
		if err != nil {
			t.Fatal(err)
		}
	}(executors)
	executors.TryExecute(ctx, RandTask)
	t.Log("goroutines", executors.Goroutines())
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
	ctx := context.Background()
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

	err := executors.CloseGracefully()
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
// BenchmarkExecutors_Execute
// BenchmarkExecutors_TryExecute-20    	 2680070	       426.4 ns/op	         0 failed	       0 B/op	       0 allocs/op
func BenchmarkExecutors_TryExecute(b *testing.B) {
	b.ReportAllocs()
	executors := rxp.New()
	ctx := context.Background()
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
	err := executors.CloseGracefully()
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
// BenchmarkExecutors_Execute
// BenchmarkExecutors_UnlimitedExecute-20    	 6444103	       186.6 ns/op	         0 failed	      34 B/op	       1 allocs/op
func BenchmarkExecutors_UnlimitedExecute(b *testing.B) {
	b.ReportAllocs()
	executors := rxp.New()
	ctx := context.Background()
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
	err := executors.CloseGracefully()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
