package async_test

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"sync/atomic"
	"testing"
	"time"
)

func prepare() (ctx context.Context, closer func() error) {
	ctx = context.Background()
	executors := rxp.New()
	ctx = rxp.With(ctx, executors)
	closer = executors.CloseGracefully
	return
}

func TestTryPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, ok := async.TryPromise[int](ctx)
	if !ok {
		t.Errorf("try promise failed")
		return
	}
	promise.Succeed(1)
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestMustPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, err := async.MustPromise[int](ctx)
	if err != nil {
		t.Errorf("try promise failed, %v", err)
		return
	}
	promise.Succeed(1)
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestTryPromise_CompleteErr(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, ok := async.TryPromise[int](ctx)
	if !ok {
		t.Errorf("try promise failed")
		return
	}
	promise.Fail(errors.New("complete failed"))
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestTryPromise_Cancel(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise1, ok1 := async.TryPromise[int](ctx)
	if !ok1 {
		t.Errorf("try promise1 failed")
		return
	}
	promise2, ok2 := async.TryPromise[int](ctx)
	if !ok2 {
		t.Errorf("try promise2 failed")
		return
	}
	promise2.Future().OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future2 entry:", result, err)
	})

	future1 := promise1.Future()
	future1.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future1 entry:", result, err)
		promise2.Succeed(2)
	})
	promise1.Cancel()
}

func TestTryPromise_Timeout(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise1, ok1 := async.TryPromise[int](ctx)
	if !ok1 {
		t.Errorf("try promise1 failed")
		return
	}
	promise1.SetDeadline(time.Now().Add(3 * time.Second))
	future1 := promise1.Future()
	future1.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future1 entry:", result, err)
		promise2, ok2 := async.TryPromise[int](ctx)
		if !ok2 {
			t.Errorf("try promise2 failed")
		}
		promise2.Succeed(2)
		future2 := promise2.Future()
		future2.OnComplete(func(ctx context.Context, result int, err error) {
			t.Log("future2 entry:", result, err)
		})
	})
	time.Sleep(4 * time.Second)
	promise1.Succeed(1)
}

// BenchmarkTryPromise
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp/async
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkTryPromise
// BenchmarkTryPromise-20    	 1969204	       558.5 ns/op	         0 failed	     110 B/op	       3 allocs/op
func BenchmarkTryPromise(b *testing.B) {
	b.ReportAllocs()
	ctx, closer := prepare()

	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			promise, ok := async.TryPromise[int](ctx)
			if !ok {
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
