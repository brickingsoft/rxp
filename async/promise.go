package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"time"
)

const (
	ns500 = 500 * time.Nanosecond
)

var (
	Canceled                = errors.New("async: promise canceled")
	DeadlineExceeded        = errors.New("async: deadline exceeded")
	Busy                    = errors.New("async: busy")
	UnexpectedContextFailed = errors.New("async: unexpected context failed")
	ExecutorsClosed         = rxp.ErrClosed
)

// IsExecutorsClosed
// 是否为 ExecutorsClosed 错误，指 rxp.Executors 关闭。
func IsExecutorsClosed(err error) bool {
	return errors.Is(err, ExecutorsClosed)
}

// IsUnexpectedContextFailed
// 是否为 UnexpectedContextFailed 错误，指 context.Context 完成了但没有错误。
func IsUnexpectedContextFailed(err error) bool {
	return errors.Is(err, UnexpectedContextFailed)
}

// IsCanceled
// 是否为 Canceled 错误，指 非 stream Promise 取消且关闭。
// 由：
// - Promise.Cancel 触发
// - context.Context 取消或超时触发，并携带 UnexpectedContextFailed
// - rxp.Executors 关闭触发
func IsCanceled(err error) bool {
	return errors.Is(err, Canceled)
}

// IsDeadlineExceeded
// 是否为 DeadlineExceeded 错误，指超时或 context.Context 超时。
// stream 不会关闭。非 stream 会关闭，所以还有一个 Canceled。
func IsDeadlineExceeded(err error) bool {
	return errors.Is(err, DeadlineExceeded)
}

// IsBusy
// 是否为 Busy 错误，指 rxp.Executors 资源不足。
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
	Complete(r R, err error) bool
	// Succeed
	// 成功完成
	Succeed(r R) bool
	// Fail
	// 错误完成
	Fail(err error) bool
	// Cancel
	// 取消许诺，未来会是一个 Canceled 错误。
	Cancel()
	// SetErrInterceptor
	// 设置错误拦截器，必须在 Complete， Succeed， Fail， Cancel 和 Future.OnComplete 前。
	SetErrInterceptor(v ErrInterceptor[R])
	// Future
	// 未来
	//
	// 注意：必须调用 Future.OnComplete，否则协程会泄漏。
	Future() (future Future[R])
}

func newDeadlineExceededError(deadline time.Time) error {
	return &DeadlineExceededError{deadline, DeadlineExceeded}
}

func AsDeadlineExceededError(err error) (*DeadlineExceededError, bool) {
	var target *DeadlineExceededError
	ok := errors.As(err, &target)
	return target, ok
}

type DeadlineExceededError struct {
	Deadline time.Time
	Err      error
}

func (e *DeadlineExceededError) Error() string {
	return e.Err.Error()
}

func (e *DeadlineExceededError) Unwrap() error { return e.Err }

func newUnexpectedContextError(ctx context.Context) error {
	return &UnexpectedContextError{ctx.Err(), UnexpectedContextFailed}
}

func AsUnexpectedContextError(err error) (*UnexpectedContextError, bool) {
	var target *UnexpectedContextError
	ok := errors.As(err, &target)
	return target, ok
}

type UnexpectedContextError struct {
	CtxErr error
	Err    error
}

func (e *UnexpectedContextError) Error() string {
	return e.Err.Error()
}

func (e *UnexpectedContextError) Unwrap() error { return e.Err }
