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
	Stream      bool
	Mode        PromiseMode
	WaitTimeout time.Duration
}

type Option func(*Options)

// WithStream
// 流式许诺
func WithStream() Option {
	return func(o *Options) {
		o.Stream = true
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
		if timeout > -1 {
			o.WaitTimeout = timeout
		}
	}
}

// WithWait
// 等待一个可用的
func WithWait() Option {
	return func(o *Options) {
		o.WaitTimeout = 0
	}
}

type optionsCtxKey struct{}

// WithOptions
// 把 Make 的 Options 绑定到 context.Context。常用于设置上下文中的默认选项。
func WithOptions(ctx context.Context, options ...Option) context.Context {
	opt := Options{
		Stream:      false,
		Mode:        Normal,
		WaitTimeout: -1,
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
			Stream:      false,
			Mode:        Normal,
			WaitTimeout: -1,
		}
	}
	opt := value.(Options)
	return opt
}

// Make
// 构建一个许诺
func Make[R any](ctx context.Context, options ...Option) (p Promise[R], err error) {
	opt := getOptions(ctx)
	for _, o := range options {
		o(&opt)
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
		exec, has := rxp.TryFrom(ctx)
		if !has {
			err = errors.New("async: executable not found")
			return
		}
		if opt.WaitTimeout < 0 {
			submitter, has = exec.TryGetTaskSubmitter()
			if !has {
				err = Busy
				return
			}
			break
		}
		var cancel context.CancelFunc
		if opt.WaitTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, opt.WaitTimeout)
		}
		times := 10
		for {
			submitter, has = exec.TryGetTaskSubmitter()
			if has {
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
		if cancel != nil {
			cancel()
		}
		break
	default:
		err = errors.New("async: invalid mode")
		return
	}
	stream := opt.Stream
	p = newFuture[R](ctx, submitter, stream)
	return
}
