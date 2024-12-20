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
	ResultTypeUnmatched     = errors.New("async: result type unmatched")
	EOF                     = errors.New("async: end of future")
	DeadlineExceeded        = errors.Join(errors.New("async: deadline exceeded"), context.DeadlineExceeded)
	UnexpectedEOF           = errors.New("async: unexpected EOF")
	Busy                    = errors.New("async: busy")
	UnexpectedContextFailed = errors.New("async: unexpected context failed")
	ExecutorsClosed         = rxp.ErrClosed
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

// IsExecutorsClosed
// 是否为 ExecutorsClosed 错误
func IsExecutorsClosed(err error) bool {
	return errors.Is(err, ExecutorsClosed)
}

// IsUnexpectedContextFailed
// 是否为 UnexpectedContextFailed 错误
func IsUnexpectedContextFailed(err error) bool {
	return errors.Is(err, UnexpectedContextFailed)
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
	// Deadline
	// 是否超时
	Deadline() (deadline time.Time, ok bool)
	// UnexpectedEOF
	// 是否存在非正常结束错误
	UnexpectedEOF() (err error)
	// Future
	// 未来
	//
	// 注意：必须调用 Future.OnComplete，否则协程会泄漏。
	Future() (future Future[R])
}

func newPromise[R any](ctx context.Context, submitter rxp.TaskSubmitter) Promise[R] {
	return newFuture[R](ctx, submitter, false)
}
