package rxp_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"sync/atomic"
	"testing"
)

func TestAlone_TryExecute(t *testing.T) {
	tasks := 100
	counter := new(atomic.Int64)
	executors, err := rxp.New(rxp.WithMode(rxp.AloneMode))
	if err != nil {
		t.Error(err)
		return
	}
	ctx := context.Background()
	defer func(executors rxp.Executors) {
		err = executors.Close()
		if err != nil {
			t.Fatal(err)
			return
		}
		t.Log("goroutines", executors.Goroutines())
		t.Log("executed", counter.Load(), counter.Load() == int64(tasks))
	}(executors)
	submitted := 0
	for i := 0; i < tasks; i++ {
		if err = executors.TryExecute(ctx, &randTask{}); err == nil {
			submitted++
		}
	}
	t.Log("submitted", submitted)
	t.Log("goroutines", executors.Goroutines())
}

func BenchmarkAlone_Execute_Parallel(b *testing.B) {
	// BenchmarkAlone_Execute_Parallel-20    	 1486959	       799.1 ns/op	         0 failed	      65 B/op	       1 allocs/op

	b.ReportAllocs()
	executors, err := rxp.New(rxp.WithMode(rxp.AloneMode))
	if err != nil {
		b.Error(err)
		return
	}
	ctx := context.Background()
	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err = executors.Execute(ctx, &randTask{})
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

func BenchmarkAlone_Execute(b *testing.B) {
	// BenchmarkAlone_Execute-20    	 4385469	       253.5 ns/op	         0 failed	      48 B/op	       1 allocs/op
	b.ReportAllocs()
	executors, err := rxp.New(rxp.WithMode(rxp.AloneMode))
	if err != nil {
		b.Error(err)
		return
	}
	ctx := context.Background()
	failed := new(atomic.Int64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = executors.Execute(ctx, &randTask{})
		if err != nil {
			failed.Add(1)
		}
	}
	err = executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
