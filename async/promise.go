package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"runtime"
	"time"
)

const (
	ns500 = 500 * time.Nanosecond
)

var (
	ResultTypeUnmatched = errors.New("async: result type unmatched")
	EOF                 = errors.New("async: end of future")
	DeadlineExceeded    = errors.Join(errors.New("async: deadline exceeded"), context.DeadlineExceeded)
	UnexpectedEOF       = errors.New("async: unexpected EOF")
	Busy                = errors.New("async: busy")
)

// IsEOF
// 是否为 EOF 错误
func IsEOF(err error) bool {
	return errors.Is(err, EOF)
}

// IsUnexpectedEOF
// 是否为 UnexpectedEOF 错误
func IsUnexpectedEOF(err error) bool {
	return errors.Is(err, UnexpectedEOF)
}

// IsCanceled
// 是否为 context.Canceled 错误
func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}

// IsDeadlineExceeded
// 是否为 DeadlineExceeded 错误
func IsDeadlineExceeded(err error) bool {
	return errors.Is(err, DeadlineExceeded)
}

// IsResultTypeUnmatched
// 是否为 ResultTypeUnmatched 错误
func IsResultTypeUnmatched(err error) bool {
	return errors.Is(err, ResultTypeUnmatched)
}

// IsBusy
// 是否为 Busy 错误
func IsBusy(err error) bool {
	return errors.Is(err, Busy)
}

// Promise
// 许诺一个未来。
//
// 注意：如果许诺了，则必须调用 Promise.Future 及 Future.OnComplete，否则协程会泄漏。
type Promise[R any] interface {
	// Complete
	// 完成
	Complete(r R, err error)
	// Succeed
	// 成功完成
	Succeed(r R)
	// Fail
	// 错误完成
	Fail(cause error)
	// Cancel
	// 取消许诺，未来会是一个 context.Canceled 错误。
	Cancel()
	// SetDeadline
	// 设置死期。
	// 当超时后，未来会是一个 context.DeadlineExceeded 错误。
	SetDeadline(t time.Time)
	// Future
	// 未来。
	//
	// 注意：必须调用 Future.OnComplete，否则协程会泄漏。
	Future() (future Future[R])
}

// TryPromise
// 尝试获取一个许诺，如果资源已耗光则获取不到。
//
// 许诺只能完成一次，完成后则不可再用。
// 当 Promise.Complete ，Promise.Succeed ，Promise.Fail 后，不必再 Promise.Cancel 来关闭它。
func TryPromise[T any](ctx context.Context) (promise Promise[T], ok bool) {
	exec := rxp.From(ctx)
	submitter, has := exec.TryGetTaskSubmitter()
	if has {
		promise = newPromise[T](ctx, submitter)
		ok = true
	}
	return
}

// MustPromise
// 必须获得一个许诺，如果资源已耗光则等待，直到可以或者上下文错误。
func MustPromise[T any](ctx context.Context) (promise Promise[T], err error) {
	times := 10
	ok := false
	for {
		promise, ok = TryPromise[T](ctx)
		if ok {
			break
		}
		if err = ctx.Err(); err != nil {
			break
		}
		time.Sleep(ns500)
		times--
		if times < 0 {
			times = 10
			runtime.Gosched()
		}
	}
	return
}

func newPromise[R any](ctx context.Context, submitter rxp.TaskSubmitter) Promise[R] {
	return newFuture[R](ctx, submitter, false)
}
