package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp"
	"runtime"
	"time"
)

type Options struct {
	WaitTimeout time.Duration
	FutureOptions
}

type Option func(*Options)

// WithStream
// 流式许诺
//
// 无限流的特性是可以无限次完成许诺，而不是一次。
//
// 但要注意，必须在不需要它后，调用 Promise.Cancel 来关闭它。
func WithStream() Option {
	return WithStreamAndSize(defaultStreamChannelSize)
}

// WithStreamAndSize
// 流式许诺
//
// 无限流的特性是可以无限次完成许诺，而不是一次。
//
// 但要注意，必须在不需要它后，调用 Promise.Cancel 来关闭它。
func WithStreamAndSize(buf int) Option {
	return func(o *Options) {
		if buf < 1 {
			buf = 1
		}
		o.StreamBuffer = buf
	}
}

// WithWaitTimeout
// 在有限时间内等待一个可用的
func WithWaitTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		if timeout < 1 {
			return
		}
		if o.WaitTimeout > 0 && o.WaitTimeout < timeout {
			return
		}
		o.WaitTimeout = timeout
	}
}

// WithWait
// 等待一个可用的
func WithWait() Option {
	return func(o *Options) {
		if o.WaitTimeout == 0 {
			o.WaitTimeout = -1
		}
	}
}

// WithDeadline
// 设置死期
func WithDeadline(deadline time.Time) Option {
	return func(o *Options) {
		if deadline.IsZero() {
			return
		}
		timeout := time.Until(deadline)
		if timeout < 1 {
			return
		}
		if o.WaitTimeout < 1 || o.WaitTimeout > timeout {
			o.WaitTimeout = timeout
		}
		o.Deadline = deadline
	}
}

// WithTimeout
// 设置超时
func WithTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		if timeout < 1 {
			return
		}
		if o.WaitTimeout < 1 || o.WaitTimeout > timeout {
			o.WaitTimeout = timeout
		}
		o.Deadline = time.Now().Add(timeout)
	}
}

type optionsCtxKey struct{}

// WithOptions
// 把 Make 的 Options 绑定到 context.Context。常用于设置上下文中的默认选项。
func WithOptions(ctx context.Context, options ...Option) context.Context {
	opt := Options{
		WaitTimeout: 0,
		FutureOptions: FutureOptions{
			StreamBuffer: 1,
			Deadline:     time.Time{},
		},
	}
	for _, o := range options {
		o(&opt)
	}
	return context.WithValue(ctx, optionsCtxKey{}, opt)
}

func getOptions(ctx context.Context) Options {
	value := ctx.Value(optionsCtxKey{})
	if value == nil {
		return Options{
			WaitTimeout: 0,
			FutureOptions: FutureOptions{
				StreamBuffer: 1,
				Deadline:     time.Time{},
			},
		}
	}
	opt := value.(Options)
	return opt
}

// Make
// 构建一个许诺。
//
// 如果 rxp.Executors 不可用，则返回 rxp.ErrBusy。
//
// 流式许诺：使用 WithStream 进行设置。
//
// 设置等待协程分配时长：使用 WithWaitTimeout 进行设置，只适用于普通模式，当超时后返回 rxp.ErrBusy。
//
// 无限等待协程分配：使用 WithWait 进行设置。
//
// 设置超时：使用 WithDeadline 或 WithTimeout，它会覆盖 WithWaitTimeout 或 WithWait。
func Make[R any](ctx context.Context, options ...Option) (p Promise[R], err error) {
	opt := getOptions(ctx)
	for _, o := range options {
		o(&opt)
	}
	// no timeout
	if opt.WaitTimeout == 0 {
		submitter, submitterErr := rxp.TryGetTaskSubmitter(ctx)
		if submitterErr != nil {
			err = errors.New("make failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(submitterErr))
			return
		}
		p = newFuture[R](ctx, submitter, opt.FutureOptions)
		return
	}

	// await or timeout
	var timer *time.Timer
	if timeout := opt.WaitTimeout; timeout > 0 {
		timer = acquireTimer(timeout)
	}
	times := 10
	stopped := false
	for {
		select {
		case <-ctx.Done():
			err = errors.New("make failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(&UnexpectedContextError{ctx.Err(), UnexpectedContextFailed}))
			stopped = true
			break
		case <-timer.C:
			err = errors.New("make failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(rxp.ErrBusy))
			stopped = true
			break
		default:
			submitter, submitterErr := rxp.TryGetTaskSubmitter(ctx)
			if submitterErr != nil {
				if IsBusy(submitterErr) {
					continue
				}
				err = errors.New("make failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(submitterErr))
				stopped = true
				break
			}
			if submitter != nil {
				p = newFuture[R](ctx, submitter, opt.FutureOptions)
				stopped = true
				break
			}
			times--
			if times < 0 {
				times = 10
				runtime.Gosched()
			}
			time.Sleep(ns500)
			break
		}
		if stopped {
			break
		}
	}
	if timer != nil {
		releaseTimer(timer)
	}
	return
}
