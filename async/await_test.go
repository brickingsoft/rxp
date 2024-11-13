package async_test

import (
	"github.com/brickingsoft/rxp/async"
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
	promise, ok := async.TryPromise[int](ctx)
	if !ok {
		t.Errorf("try promise failed")
		return
	}
	promise.Succeed(1)
	future := promise.Future()
	v, err := async.Await[int](future)
	if err != nil {
		t.Errorf("await failed: %v", err)
	}
	t.Log(v)
}
