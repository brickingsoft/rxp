package async_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMake(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, err := async.Make[int](ctx)
	if err != nil {
		t.Errorf("try promise failed")
		return
	}
	promise.Succeed(1)
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestCloseAfterMake(t *testing.T) {
	ctx := context.Background()
	executors := rxp.New(rxp.WithCloseTimeout(500 * time.Millisecond))
	ctx = rxp.With(ctx, executors)
	promise, promiseErr := async.Make[int](ctx)
	if promiseErr != nil {
		t.Errorf("try promise failed")
		return
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(promise async.Promise[int], wg *sync.WaitGroup) {
		future := promise.Future()
		future.OnComplete(func(ctx context.Context, result int, err error) {
			t.Log("future entry:", result, err, async.IsExecutorsClosed(err))
			wg.Done()
		})
		_ = executors.CloseGracefully()
		promise.Succeed(1)
	}(promise, wg)

	wg.Wait()
}

// BenchmarkMake
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp/async
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkMake
// BenchmarkMake-20    	 2016514	       533.3 ns/op	         0 failed	     175 B/op	       4 allocs/op
func BenchmarkMake(b *testing.B) {
	b.ReportAllocs()
	ctx, closer := prepare()

	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			promise, err := async.Make[int](ctx)
			if err != nil {
				failed.Add(1)
				return
			}
			promise.Succeed(1)
			promise.Future().OnComplete(func(ctx context.Context, result int, err error) {
			})
		}
	})
	err := closer()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkMakeDirect
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp/async
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkMakeDirect
// BenchmarkMakeDirect-20    	 2740728	       449.4 ns/op	         0 failed	     230 B/op	       6 allocs/op
func BenchmarkMakeDirect(b *testing.B) {
	b.ReportAllocs()
	ctx, closer := prepare()

	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			promise, err := async.Make[int](ctx, async.WithDirectMode())
			if err != nil {
				failed.Add(1)
				return
			}
			promise.Succeed(1)
			promise.Future().OnComplete(func(ctx context.Context, result int, err error) {
			})
		}
	})
	err := closer()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkMakeUnlimited
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp/async
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkMakeUnlimited
// BenchmarkMakeUnlimited-20    	 2942133	       424.2 ns/op	         0 failed	     225 B/op	       6 allocs/op
func BenchmarkMakeUnlimited(b *testing.B) {
	b.ReportAllocs()
	ctx, closer := prepare()

	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			promise, err := async.Make[int](ctx, async.WithUnlimitedMode())
			if err != nil {
				failed.Add(1)
				return
			}
			promise.Succeed(1)
			promise.Future().OnComplete(func(ctx context.Context, result int, err error) {
			})
		}
	})
	err := closer()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
