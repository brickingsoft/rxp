package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"runtime"
	"time"
)

type PromiseMode int

const (
	Normal PromiseMode = iota
	Unlimited
	Direct
)

type Options struct {
	Mode        PromiseMode
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
//
// 关于许诺值，如果它实现了 io.Closer ，则当 Promise.Cancel 后且它未没处理，那么会自动 转化为 io.Closer 进行关闭。
//
// 由于在关闭后依旧可以完成许诺，因此所许诺的内容如果含有关闭功能，则请实现 io.Closer。
func WithStream() Option {
	return WithStreamAndBuffer(64)
}

// WithStreamAndBuffer
// 流式许诺
func WithStreamAndBuffer(buffer int) Option {
	return func(o *Options) {
		o.Stream = true
		if buffer <= 0 {
			buffer = 1
		}
		o.BufferSize = buffer
	}
}

// WithNormalMode
// 普通模式
func WithNormalMode() Option {
	return func(o *Options) {
		o.Mode = Normal
	}
}

// WithUnlimitedMode
// 无限制模式
func WithUnlimitedMode() Option {
	return func(o *Options) {
		o.Mode = Unlimited
	}
}

// WithDirectMode
// 直接模式
func WithDirectMode() Option {
	return func(o *Options) {
		o.Mode = Direct
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
		o.Timeout = timeout
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
		o.Timeout = timeout
	}
}

type optionsCtxKey struct{}

// WithOptions
// 把 Make 的 Options 绑定到 context.Context。常用于设置上下文中的默认选项。
func WithOptions(ctx context.Context, options ...Option) context.Context {
	opt := Options{
		Mode:        Normal,
		WaitTimeout: 0,
		FutureOptions: FutureOptions{
			Stream:     false,
			BufferSize: 0,
			Timeout:    0,
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
			Mode:        Normal,
			WaitTimeout: 0,
			FutureOptions: FutureOptions{
				Stream:     false,
				BufferSize: 0,
				Timeout:    0,
			},
		}
	}
	opt := value.(Options)
	return opt
}

// Make
// 构建一个许诺
//
// 流式许诺：使用 WithStream 进行设置。
//
// 普通模式：使用 WithNormalMode 进行设置，这是默认的。
//
// 无限制模式：使用 WithUnlimitedMode 进行设置。
//
// 直接模式：使用 WithDirectMode 进行设置。
//
// 设置等待协程分配时长：使用 WithWaitTimeout 进行设置，只适用于普通模式，当超时后返回 Busy。
//
// 无限等待协程分配：使用 WithWait 进行设置，只适用于普通模式。
//
// 设置超时：使用 WithDeadline 或 WithTimeout，它会覆盖 WithWaitTimeout 或 WithWait。
func Make[R any](ctx context.Context, options ...Option) (p Promise[R], err error) {
	opt := getOptions(ctx)
	for _, o := range options {
		o(&opt)
	}
	exec, has := rxp.TryFrom(ctx)
	if !has {
		err = errors.New("async: executable not found")
		return
	}
	if !exec.Running() {
		err = ExecutorsClosed
		return
	}
	var submitter rxp.TaskSubmitter
	switch opt.Mode {
	case Unlimited:
		submitter = &unlimitedSubmitter{ctx: ctx}
		break
	case Direct:
		submitter = &directSubmitter{ctx: ctx}
		break
	case Normal:
		if opt.WaitTimeout == 0 {
			submitter, has = exec.TryGetTaskSubmitter()
			if !has {
				if !exec.Running() {
					err = ExecutorsClosed
				} else {
					err = Busy
				}
				return
			}
			break
		}
		var waitCtx context.Context
		var cancel context.CancelFunc
		if opt.WaitTimeout > 0 {
			waitCtx, cancel = context.WithTimeout(ctx, opt.WaitTimeout)
		}
		times := 10
		for {
			submitter, has = exec.TryGetTaskSubmitter()
			if has {
				break
			}
			if waitCtx != nil && waitCtx.Err() != nil {
				err = Busy
				break
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				err = newUnexpectedContextError(ctx)
				break
			}
			if !exec.Running() {
				err = ExecutorsClosed
				break
			}
			times--
			if times < 0 {
				times = 10
				runtime.Gosched()
			}
			time.Sleep(ns500)
		}
		if cancel != nil {
			cancel()
		}
		if err != nil {
			return
		}
		break
	default:
		err = errors.New("async: invalid mode")
		return
	}
	p = newFuture[R](ctx, submitter, opt.FutureOptions)
	return
}
