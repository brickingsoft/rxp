package async_test

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/async"
	"testing"
	"time"
)

func TestTryPromise_CompleteErr(t *testing.T) {
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)
	defer func() {
		err := exec.Close()
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
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)
	defer func() {
		err := exec.Close()
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
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)
	defer func() {
		err := exec.Close()
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
	exec, execErr := rxp.New()
	if execErr != nil {
		t.Fatal(execErr)
		return
	}
	ctx := context.Background()
	ctx = rxp.With(ctx, exec)
	defer func() {
		err := exec.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	promise, promiseErr := async.Make[int](ctx)
	if promiseErr != nil {
		t.Errorf("try promise failed")
		return
	}
	promise.SetErrInterceptor(func(ctx context.Context, value int, err error) (future async.Future[int]) {
		t.Log(err)
		future = async.FailedImmediately[int](ctx, err)
		return
	})
	promise.Fail(errors.New("complete failed"))
	future := promise.Future()
	future.OnComplete(func(ctx context.Context, result int, err error) {
		t.Log("future entry:", result, err)
	})
}
