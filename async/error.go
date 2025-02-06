package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp"
)

var (
	Canceled                = errors.Define("promise canceled")
	DeadlineExceeded        = errors.Define("deadline exceeded")
	Busy                    = errors.Define("busy")
	UnexpectedContextFailed = errors.Define("unexpected context failed")
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
	if errors.Is(err, UnexpectedContextFailed) {
		return true
	}
	var v *UnexpectedContextError
	if errors.As(err, &v) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	return false
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
	if errors.Is(err, DeadlineExceeded) {
		return true
	}
	var v *DeadlineExceededError
	return errors.As(err, &v)
}

// IsBusy
// 是否为 Busy 错误，指 rxp.Executors 资源不足。
func IsBusy(err error) bool {
	return errors.Is(err, Busy)
}

const (
	errMetaPkgKey = "pkg"
	errMetaPkgVal = "async"
)
