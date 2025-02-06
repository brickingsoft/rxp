package async

import (
	"github.com/brickingsoft/errors"
	"time"
)

const (
	ns500 = 500 * time.Nanosecond
)

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
	// SetUnhandledResultHandler
	// 设置未处理结果的处理器，必须在 Complete， Succeed， Fail， Cancel 和 Future.OnComplete 前。
	// 当 Complete， Succeed， Fail 后。但结果因 ExecutorsClosed 或其它原因未被处理时触发。
	SetUnhandledResultHandler(handler UnhandledResultHandler[R])
	// Future
	// 未来
	//
	// 注意：必须调用 Future.OnComplete，否则协程会泄漏。
	Future() (future Future[R])
}

func AsDeadlineExceededError(err error) (*DeadlineExceededError, bool) {
	var target *DeadlineExceededError
	ok := errors.As(err, &target)
	return target, ok
}

// DeadlineExceededError
// 超时错误
type DeadlineExceededError struct {
	// Deadline
	// 超时时间
	Deadline time.Time
	// Err
	// 超时错误，常为 DeadlineExceeded
	Err error
}

func (e *DeadlineExceededError) Error() string {
	return e.Err.Error()
}

func (e *DeadlineExceededError) Unwrap() error { return e.Err }

// AsUnexpectedContextError
// 转为 UnexpectedContextError
func AsUnexpectedContextError(err error) (*UnexpectedContextError, bool) {
	var target *UnexpectedContextError
	ok := errors.As(err, &target)
	return target, ok
}

// UnexpectedContextError
// 上下文错误
type UnexpectedContextError struct {
	// CtxErr
	// 值为 context.Context 的 Err()
	CtxErr error
	// Err
	// 错误，常为 UnexpectedContextFailed
	Err error
}

func (e *UnexpectedContextError) Error() string {
	return e.Err.Error()
}

func (e *UnexpectedContextError) Unwrap() error { return e.Err }
