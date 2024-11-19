package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"sync/atomic"
	"testing"
)

func TestDirectPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise := async.DirectPromise[int](ctx)
	promise.Succeed(1)
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestDirectStreamPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise := async.DirectStreamPromise[*Closer](ctx)

	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result *Closer, err error) {
		t.Log("future entry:", result, err)
		if err != nil {
			t.Log("is closed:", async.IsEOF(err))
			return
		}
		return
	})
	for i := 0; i < 10; i++ {
		promise.Succeed(&Closer{N: i, t: t})
	}
	promise.Cancel()
	for i := 0; i < 10; i++ {
		promise.Succeed(&Closer{N: i, t: t})
	}
}

func TestDirectStreamPromises(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()

	promise, promiseErr := async.DirectStreamPromises[int](ctx, 10)
	if promiseErr != nil {
		t.Error(promiseErr)
		return
	}
	promise.Future().OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
		if err != nil {
			t.Log("is closed:", async.IsEOF(err))
			return
		}
		return
	})
	for i := 0; i < 10; i++ {
		promise.Succeed(i)
	}
	promise.Cancel()
}

// BenchmarkDirectPromise
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp/async
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkDirectPromise
// BenchmarkDirectPromise-20    	 3813390	       272.3 ns/op	         0 failed	     215 B/op	       6 allocs/op
func BenchmarkDirectPromise(b *testing.B) {
	b.ReportAllocs()
	ctx, closer := prepare()

	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			promise := async.DirectPromise[int](ctx)
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
