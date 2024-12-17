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
//
// 无限流的特性是可以无限次完成许诺，而不是一次。
//
// 但要注意，必须在不需要它后，调用 Promise.Cancel 来关闭它。
//
// 关于许诺值，如果它实现了 io.Closer ，则当 Promise.Cancel 后且它未没处理，那么会自动 转化为 io.Closer 进行关闭。
//
// 由于在关闭后依旧可以完成许诺，因此所许诺的内容如果含有关闭功能，则请实现 io.Closer。
func WithStream() Option {
	return func(o *Options) {
		o.Stream = true
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
//
// 流式许诺：使用 WithStream 进行设置。
//
// 普通模式：使用 WithNormalMode 进行设置，这是默认的。
//
// 无限制模式：使用 WithUnlimitedMode 进行设置。
//
// 直接模式：使用 WithDirectMode 进行设置。
//
// 设置等待协程分配时长：使用 WithWaitTimeout 进行设置，只适用于普通模式。
//
// 无限等待协程分配：使用 WithWait 进行设置，只适用于普通模式。
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
			if !exec.Running() {
				err = ExecutorsClosed
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
		if err != nil {
			return
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
