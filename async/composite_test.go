package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"testing"
)

func TestComposite(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()

	promises := make([]async.Promise[int], 0, 1)
	for i := 0; i < 10; i++ {
		promise, ok := async.TryPromise[int](ctx)
		if !ok {
			t.Errorf("try promise failed")
			return
		}
		promises = append(promises, promise)
	}

	composite := async.Composite[[]async.Result[int]](ctx, promises)
	composite.OnComplete(func(ctx context.Context, results []async.Result[int], err error) {
		nn := make([]int, 0, 1)
		ee := make([]error, 0, 1)
		for _, result := range results {
			if result.Succeed() {
				nn = append(nn, result.Entry())
			} else {
				ee = append(ee, result.Cause())
			}
		}
		t.Log("composite future:", nn, ee)
	})

	for i, promise := range promises {
		promise.Succeed(i)
	}

}
