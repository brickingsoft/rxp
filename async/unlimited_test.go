package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"testing"
)

func TestUnlimitedPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()

	promise, _ := async.Make[int](ctx, async.WithUnlimitedMode())
	promise.Succeed(1)
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestUnlimitedStreamPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, _ := async.Make[*Closer](ctx, async.WithUnlimitedMode(), async.WithStream())
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
