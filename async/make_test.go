package async_test

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMake(t *testing.T) {
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := exec.Context()
	defer func() {
		err := exec.Close()
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
	exec, execErr := rxp.New(rxp.WithCloseTimeout(50 * time.Millisecond))
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := exec.Context()
	defer func() {
		err := exec.Close()
		if err == nil {
			t.Error("not closed")
		}
	}()

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
			t.Log("future entry:", result, async.IsExecutorsClosed(err), err)
			wg.Done()
		})
		go func() {
			_ = exec.Close()
		}()
		time.Sleep(500 * time.Millisecond)
		promise.Succeed(1)
	}(promise, wg)
	wg.Wait()
}

func TestFailedWithResult(t *testing.T) {
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := exec.Context()
	defer func() {
		err := exec.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	promise, err := async.Make[int](ctx)
	if err != nil {
		t.Errorf("try promise failed")
		return
	}
	promise.Complete(1, errors.New("error"))
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

// BenchmarkMake_Share
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp/async
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkMake_Share
// BenchmarkMake_Share-20    	 2270600	       485.9 ns/op	         0 failed	     137 B/op	       2 allocs/op
func BenchmarkMake_Share(b *testing.B) {
	b.ReportAllocs()

	exec, execErr := rxp.New()
	if execErr != nil {
		b.Fatal(execErr)
		return
	}
	ctx := exec.Context()

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
	err := exec.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}

// BenchmarkMake_Alone
// goos: windows
// goarch: amd64
// pkg: github.com/brickingsoft/rxp/async
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
// BenchmarkMake_Alone
// BenchmarkMake_Alone-20    	 3627508	       290.2 ns/op	         0 failed	     201 B/op	       3 allocs/op
func BenchmarkMake_Alone(b *testing.B) {
	b.ReportAllocs()

	exec, execErr := rxp.New(rxp.WithMode(rxp.AloneMode))
	if execErr != nil {
		b.Fatal(execErr)
		return
	}
	ctx := exec.Context()

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
	err := exec.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
