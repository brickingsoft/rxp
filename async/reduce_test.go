package async_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"sync"
	"testing"
)

func TestReduce(t *testing.T) {
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

	wg := new(sync.WaitGroup)

	promises := make([]async.Promise[int], 0, 1)
	futures := make([]async.Future[int], 0, 1)
	for i := 0; i < 10; i++ {
		promise, ok := async.Make[int](ctx)
		if ok != nil {
			t.Errorf("try promise failed")
			return
		}
		promises = append(promises, promise)
		futures = append(futures, promise.Future())
	}

	future := async.Reduce[[]async.Result[int]](ctx, futures)

	wg.Add(1)
	future.OnComplete(func(ctx context.Context, results []async.Result[int], err error) {
		nn := make([]int, 0, 1)
		ee := make([]error, 0, 1)
		for _, result := range results {
			if cause := result.Error(); cause != nil {
				ee = append(ee, cause)
			} else {
				nn = append(nn, result.Value())
			}
		}
		t.Log("reduce futures:", nn, ee)
		wg.Done()
	})

	for i, promise := range promises {
		promise.Succeed(i)
	}

	wg.Wait()

}
