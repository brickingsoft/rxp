package async_test

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"testing"
	"time"
)

func prepare() (ctx context.Context, closer func() error) {
	ctx = context.Background()
	executors := rxp.New()
	ctx = rxp.With(ctx, executors)
	closer = executors.CloseGracefully
	return
}

func TestTryPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, promiseErr := async.Make[int](ctx)
	if promiseErr != nil {
		t.Errorf("try promise failed")
		return
	}
	promise.Succeed(1)
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestMustPromise(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, promiseErr := async.Make[int](ctx, async.WithWait())
	if promiseErr != nil {
		t.Errorf("try promise failed, %v", promiseErr)
		return
	}
	promise.Succeed(1)
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestTryPromise_CompleteErr(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise, promiseErr := async.Make[int](ctx)
	if promiseErr != nil {
		t.Errorf("try promise failed")
		return
	}
	promise.Fail(errors.New("complete failed"))
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

func TestTryPromise_Cancel(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise1, ok1 := async.Make[int](ctx)
	if ok1 != nil {
		t.Errorf("try promise1 failed")
		return
	}
	promise2, ok2 := async.Make[int](ctx)
	if ok2 != nil {
		t.Errorf("try promise2 failed")
		return
	}
	promise2.Future().OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future2 entry:", result, err)
	})

	future1 := promise1.Future()
	future1.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future1 entry:", result, err)
		promise2.Succeed(2)
	})
	promise1.Cancel()
}

func TestTryPromise_Timeout(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	promise1, ok1 := async.Make[int](ctx)
	if ok1 != nil {
		t.Errorf("try promise1 failed")
		return
	}
	promise1.SetDeadline(time.Now().Add(3 * time.Second))
	future1 := promise1.Future()
	future1.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future1 entry:", result, err)
		promise2, ok2 := async.Make[int](ctx)
		if ok2 != nil {
			t.Errorf("try promise2 failed")
			return
		}
		promise2.Succeed(2)
		future2 := promise2.Future()
		future2.OnComplete(func(ctx context.Context, result int, err error) {
			t.Log("future2 entry:", result, err)
		})
	})
	time.Sleep(4 * time.Second)
	deadline, ok := promise1.Deadline()
	t.Log("deadline:", deadline, ok)
	t.Log("UnexpectedEOF:", promise1.UnexpectedEOF())
	promise1.Succeed(1)
}
