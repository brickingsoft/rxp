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

func WithStream() Option {
	return func(o *Options) {
		o.Stream = true
	}
}

func WithUnlimitedMode() Option {
	return func(o *Options) {
		o.Mode = Unlimited
	}
}

func WithDirectMode() Option {
	return func(o *Options) {
		o.Mode = Direct
	}
}

func WithWaitTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		if timeout > -1 {
			o.WaitTimeout = timeout
		}
	}
}

func WithWait() Option {
	return func(o *Options) {
		o.WaitTimeout = 0
	}
}

func Make[R any](ctx context.Context, options ...Option) (p Promise[R], err error) {
	opt := Options{
		Stream:      false,
		Mode:        Normal,
		WaitTimeout: -1,
	}
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
