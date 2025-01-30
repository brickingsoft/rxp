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
	promise1, ok1 := async.Make[int](ctx, async.WithTimeout(time.Second*3))
	if ok1 != nil {
		t.Errorf("try promise1 failed")
		return
	}
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
	promise1.Succeed(1)
}

func TestAsDeadlineExceededError(t *testing.T) {
	v := &async.DeadlineExceededError{Err: async.DeadlineExceeded, Deadline: time.Now()}
	err, ok := async.AsDeadlineExceededError(v)
	if ok {
		t.Log("AsDeadlineExceededError:", err)
	} else {
		t.Log("AsDeadlineExceededError: failed")
	}
}

func TestAsUnexpectedContextError(t *testing.T) {
	v := &async.UnexpectedContextError{CtxErr: errors.New("some context"), Err: async.UnexpectedContextFailed}
	err, ok := async.AsUnexpectedContextError(v)
	if ok {
		t.Log("AsUnexpectedContextError:", err)
	} else {
		t.Log("AsUnexpectedContextError: failed")
	}
}

func TestStreamPromises_WithErrInterceptor(t *testing.T) {
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
	promise.WithErrInterceptor(&errInterceptor[int]{t: t})
	promise.Fail(errors.New("complete failed"))
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}

type errInterceptor[R any] struct {
	t *testing.T
}

func (e *errInterceptor[R]) Handle(ctx context.Context, value R, err error) (future async.Future[R]) {
	e.t.Log(err)
	future = async.FailedImmediately[R](ctx, err)
	return
}

func TestStreamPromises_SetResultChan(t *testing.T) {
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
	ch := make(chan async.Result[int], 1)
	setErr := promise.SetResultChan(ch)
	if setErr != nil {
		t.Error(setErr)
		return
	}
	promise.Future().OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
	ch <- async.SucceedResult[int](1)
	close(ch)
}
