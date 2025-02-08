package async_test

import (
	"context"
	"github.com/brickingsoft/rxp"
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

	promise, promiseErr := async.Make[*Closer](ctx, async.WithStream())
	if promiseErr != nil {
		t.Errorf("try promise failed")
		return
	}
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result *Closer, err error) {
		t.Log("future entry:", result, err)
		if err != nil {
			t.Log("is canceled:", async.IsCanceled(err))
			return
		}
		return
	})
	for i := 0; i < 10; i++ {
		if ok := promise.Succeed(&Closer{N: i, t: t}); !ok {
			t.Errorf("try promise failed")
		}
	}
	promise.Cancel()
	for i := 0; i < 10; i++ {
		promise.Succeed(&Closer{N: i, t: t})
	}
}
