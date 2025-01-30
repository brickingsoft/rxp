package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
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
	promise, _ := async.Make[int](ctx, async.WithDirectMode())
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
	promise, _ := async.Make[*Closer](ctx, async.WithDirectMode(), async.WithWait(), async.WithStream())

	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result *Closer, err error) {
		t.Log("future entry:", result, err)
		if err != nil {
			t.Log("is closed:", async.IsCanceled(err))
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

	promise, promiseErr := async.Make[int](ctx, async.WithDirectMode(), async.WithWait(), async.WithStream())
	if promiseErr != nil {
		t.Error(promiseErr)
		return
	}
	promise.Future().OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
		if err != nil {
			t.Log("is closed:", async.IsCanceled(err))
			return
		}
		return
	})
	for i := 0; i < 10; i++ {
		promise.Succeed(i)
	}
	promise.Cancel()
}
