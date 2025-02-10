package rxp

import "github.com/brickingsoft/errors"

const (
	errMetaPkgKey = "pkg"
	errMetaPkgVal = "rxp"
)

var (
	// ErrClosed 执行池已关闭
	ErrClosed = errors.Define("executors has been closed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
	// ErrCloseFailed 关闭执行池失败（一般是关闭超时引发）
	ErrCloseFailed = errors.Define("executors close failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
	// ErrBusy 无可用协程
	ErrBusy = errors.Define("executors are busy", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
)

// IsClosed
// 是否为 ErrClosed 错误
func IsClosed(err error) bool {
	return errors.Is(err, ErrClosed)
}

// IsBusy
// 是否为 ErrBusy 错误
func IsBusy(err error) bool {
	return errors.Is(err, ErrBusy)
}
