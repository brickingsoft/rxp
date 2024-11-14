package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"testing"
)

func TestGroup(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	futures := make([]async.Future[int], 0, 1)
	for i := 0; i < 10; i++ {
		promise, _ := async.TryPromise[int](ctx)
		promise.Succeed(i)
		futures = append(futures, promise.Future())
	}
	group := async.Group[int](futures)
	group.OnComplete(func(ctx context.Context, entry int, cause error) {
		t.Log(entry, cause)
	})
}
