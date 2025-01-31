package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"testing"
)

func TestJoinStreamFutures(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	futures := make([]async.Future[int], 0, 1)
	for i := 0; i < 10; i++ {
		promise, _ := async.Make[int](ctx, async.WithStream())
		promise.Succeed(i)
		promise.Succeed(i + 10)
		promise.Cancel()
		futures = append(futures, promise.Future())
	}
	streams := async.JoinStreamFutures[int](futures)

	streams.OnComplete(func(ctx context.Context, entry int, cause error) {
		t.Log(entry, cause)
	})

}

func TestStreamPromises(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, err := async.StreamPromises[int](ctx, 2)
	if err != nil {
		t.Error(err)
		return
	}
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, entry int, err error) {
		t.Log(entry, err)
	})
	for i := 0; i < 10; i++ {
		promise.Succeed(i)
	}
	promise.Cancel()
}
