package async_test

import (
	"errors"
	"github.com/brickingsoft/rxp/async"
	"sync"
	"testing"
)

func TestPromise_Await(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
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
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, ok := async.Make[int](ctx, async.WithStream())
	if ok != nil {
		t.Errorf("try promise failed")
		return
	}
	af := async.AwaitableFuture[int](promise.Future())
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(af async.Awaitable[int], wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			v, err := af.Await()
			if err != nil {
				if errors.Is(err, async.EOF) {
					return
				}
				t.Errorf("await failed: %v", err)
				return
			}
			t.Log("await:", v)
		}

	}(af, wg)
	for i := 0; i < 10; i++ {
		promise.Succeed(i)
	}
	promise.Cancel()
	wg.Wait()
}
