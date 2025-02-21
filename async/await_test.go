package async_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAwait(t *testing.T) {
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)
	defer func() {
		err := exec.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	promise, ok := async.Make[int](ctx)
	if ok != nil {
		t.Errorf("try promise failed")
		return
	}
	promise.Succeed(1)
	aw := async.AwaitableFuture[int](promise.Future())
	v, err := aw.Await()
	if err != nil {
		t.Errorf("await failed: %v", err)
	}
	t.Log(v)
}

func TestPromise_Await(t *testing.T) {
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)
	defer func() {
		err := exec.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	promise, ok := async.Make[int](ctx)
	if ok != nil {
		t.Errorf("try promise failed")
		return
	}
	promise.Succeed(1)
	af := async.AwaitableFuture[int](promise.Future())
	v, err := af.Await()
	if err != nil {
		t.Errorf("await failed: %v", err)
	}
	t.Log(v)
}

func TestStreamPromise_Await(t *testing.T) {
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)
	defer func() {
		err := exec.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	promise, ok := async.Make[int](ctx, async.WithStream())
	if ok != nil {
		t.Errorf("try promise failed")
		return
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	aw := async.AwaitableFuture[int](promise.Future())
	go func(aw async.Awaitable[int], wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			v, err := aw.Await()
			if err != nil {
				if async.IsCanceled(err) {
					return
				}
				t.Errorf("await failed: %v", err)
				return
			}
			t.Log("await:", v)
		}

	}(aw, wg)
	for i := 0; i < 10; i++ {
		promise.Succeed(i)
	}
	promise.Cancel()
	wg.Wait()
}

func BenchmarkAwaitableFuture(b *testing.B) {
	b.ReportAllocs()

	exec, execErr := rxp.New()
	if execErr != nil {
		b.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)

	failed := new(atomic.Int64)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			promise, err := async.Make[int](ctx)
			if err != nil {
				failed.Add(1)
				return
			}
			aw := async.AwaitableFuture[int](promise.Future())
			promise.Succeed(1)
			if _, err = aw.Await(); err != nil {
				failed.Add(1)
				return
			}
		}
	})
	err := exec.Close()
	if err != nil {
		b.Error(err)
	}
	b.ReportMetric(float64(failed.Load()), "failed")
}
