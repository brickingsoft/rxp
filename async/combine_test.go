package async_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"sync"
	"testing"
)

func TestCombine(t *testing.T) {
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

	promises := make([]async.Promise[int], 0, 1)
	futures := make([]async.Future[int], 0, 1)
	for i := 0; i < 2; i++ {
		promise, _ := async.Make[int](ctx, async.WithStream())
		promises = append(promises, promise)
		futures = append(futures, promise.Future())
	}

	combined := async.Combine[int](ctx, futures)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	combined.OnComplete(func(ctx context.Context, result int, err error) {
		if err != nil {
			if async.IsCanceled(err) {
				t.Log(err)
				wg.Done()
			} else {
				t.Error(err)
			}
			return
		}
		t.Log(result)
	})

	for _, promise := range promises {
		for i := 0; i < 5; i++ {
			promise.Succeed(i)
		}
		promise.Cancel()
	}

	wg.Wait()

}
