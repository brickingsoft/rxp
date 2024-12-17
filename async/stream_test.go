package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"testing"
)

type Closer struct {
	N int
	t *testing.T
}

func (c *Closer) Close() error {
	c.t.Log("close ", c.N)
	return nil
}

func TestTryStreamPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, promiseErr := async.Make[*Closer](ctx, async.WithStream())
	if promiseErr != nil {
		t.Errorf("try promise failed")
		return
	}
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result *Closer, err error) {
		t.Log("future entry:", result, err)
		if err != nil {
			t.Log("is eof:", async.IsEOF(err))
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

func TestMustStreamPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, err := async.Make[*Closer](ctx, async.WithStream(), async.WithWait())
	if err != nil {
		t.Errorf("try promise failed, %v", err)
		return
	}
	future := promise.Future()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	future.OnComplete(func(ctx context.Context, result *Closer, err error) {
		t.Log("future entry:", result, err)
		if err != nil {
			t.Log("is eof:", async.IsEOF(err))
			cancel()
			return
		}
		return
	})
	for i := 0; i < 10; i++ {
		promise.Succeed(&Closer{N: i, t: t})
	}
	promise.Cancel()
	<-ctx.Done()
}
