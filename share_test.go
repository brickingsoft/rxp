package rxp_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"sync/atomic"
	"testing"
)

func TestShare_TryExecute(t *testing.T) {
	tasks := 100
	counter := new(atomic.Int64)
	executors, err := rxp.New(rxp.WithMode(rxp.ShareMode))
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
		if executors.TryExecute(ctx, &randTask{}) {
			submitted++
		}
	}
	t.Log("submitted", submitted)
	t.Log("goroutines", executors.Goroutines())
}

func BenchmarkShare_Execute_Parallel(b *testing.B) {
	// BenchmarkShare_Execute_Parallel-20    	 2791100	       424.8 ns/op	         0 failed	       0 B/op	       0 allocs/op

	b.ReportAllocs()
	executors, err := rxp.New(rxp.WithMode(rxp.ShareMode))
	if err != nil {
		b.Error(err)
		return
	}
	ctx := executors.Context()
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

func BenchmarkShare_Execute(b *testing.B) {
	// BenchmarkShare_Execute-20    	 2988993	       337.9 ns/op	         0 failed	       0 B/op	       0 allocs/op
	b.ReportAllocs()
	executors, err := rxp.New(rxp.WithMode(rxp.ShareMode))
	if err != nil {
		b.Error(err)
		return
	}
	ctx := executors.Context()
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

type PipeTask struct {
	ch chan int
}

func (task *PipeTask) Handle(ctx context.Context) {
	task.ch <- 1
}

func BenchmarkShare_Execute_PipeTask(b *testing.B) {
	// BenchmarkShare_Execute-20    	 2988993	       337.9 ns/op	         0 failed	       0 B/op	       0 allocs/op
	b.ReportAllocs()
	executors, err := rxp.New(rxp.WithMode(rxp.ShareMode))
	if err != nil {
		b.Error(err)
		return
	}
	ctx := executors.Context()
	failed := new(atomic.Int64)
	task := &PipeTask{ch: make(chan int, 1)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = executors.Execute(ctx, task)
		if err != nil {
			failed.Add(1)
		} else {
			<-task.ch
		}
	}
	err = executors.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
