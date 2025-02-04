package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"time"
)

// Future
// 许诺的未来，注册一个异步非堵塞的结果处理器。
type Future[R any] interface {
	// OnComplete
	// 注册一个结果处理器，它是异步非堵塞的。
	OnComplete(handler ResultHandler[R])
}

type FutureOptions struct {
	StreamBuffer int
	Timeout      time.Duration
}

func newFuture[R any](ctx context.Context, submitter rxp.TaskSubmitter, opts FutureOptions) *futureImpl[R] {
	buffer := opts.StreamBuffer
	if buffer < 1 {
		buffer = 1
	}
	ch := acquireChannel(buffer)
	if timeout := opts.Timeout; timeout > 0 {
		ch.setTimeout(timeout)
	}
	f := &futureImpl[R]{
		channel:   ch,
		ctx:       ctx,
		submitter: submitter,
	}
	return f
}

type futureImpl[R any] struct {
	*channel
	ctx                    context.Context
	submitter              rxp.TaskSubmitter
	handler                ResultHandler[R]
	errInterceptor         ErrInterceptor[R]
	unhandledResultHandler UnhandledResultHandler[R]
}

func (f *futureImpl[R]) handleUnhandledResult() {
	if remain := f.remain(); remain > 0 {
		for i := 0; i < remain; i++ {
			v := f.get()
			if v != nil && f.unhandledResultHandler != nil {
				r, ok := v.(result[R])
				if ok {
					f.unhandledResultHandler(r.value)
				}
			}
		}
	}
}

func (f *futureImpl[R]) handle(ctx context.Context) {
	for {
		e, err := f.receive(ctx)
		if err != nil {
			if f.errInterceptor != nil {
				f.errInterceptor(ctx, *(new(R)), err).OnComplete(f.handler)
			} else {
				f.handler(f.ctx, *(new(R)), err)
			}
			if f.ra.Load() {
				continue
			}
			break
		}
		// handle result
		r, ok := e.(result[R])
		if !ok {
			err = errors.Join(Canceled, errors.New("type of result is unexpected"))
			if f.errInterceptor != nil {
				f.errInterceptor(ctx, *(new(R)), err).OnComplete(f.handler)
			} else {
				f.handler(f.ctx, *(new(R)), err)
			}
			f.end()
			break
		}
		rVal := r.Value()
		rErr := r.Error()
		if rErr != nil && f.errInterceptor != nil {
			f.errInterceptor(ctx, rVal, rErr).OnComplete(f.handler)
		} else {
			f.handler(f.ctx, rVal, rErr)
		}
		// check canceled
		if IsCanceled(rErr) {
			f.end()
		}
		if !f.ra.Load() {
			break
		}
	}
	// try unhandled
	f.handleUnhandledResult()
	// release
	releaseChannel(f.channel)
}

func (f *futureImpl[R]) OnComplete(handler ResultHandler[R]) {
	if handler == nil {
		panic(errors.New("async.Future: handler is nil"))
		return
	}
	if f.handler != nil {
		panic(errors.New("async.Future: handler already set"))
		return
	}
	ctx := f.ctx
	f.handler = handler
	if ok := f.submitter.Submit(ctx, f.handle); !ok {
		// close
		f.end()
		// try unhandled
		f.handleUnhandledResult()
		// handle
		err := errors.Join(Canceled, ExecutorsClosed)
		if f.errInterceptor != nil {
			f.errInterceptor(ctx, *(new(R)), err).OnComplete(f.handler)
		} else {
			f.handler(ctx, *(new(R)), err)
		}
		return
	}
	return
}

func (f *futureImpl[R]) Complete(r R, err error) bool {
	return f.send(result[R]{value: r, err: err})
}

func (f *futureImpl[R]) Succeed(r R) bool {
	return f.Complete(r, nil)
}

func (f *futureImpl[R]) Fail(err error) bool {
	return f.Complete(*(new(R)), err)
}

func (f *futureImpl[R]) Cancel() {
	f.send(result[R]{err: Canceled})
	f.disableSend()
}

func (f *futureImpl[R]) SetErrInterceptor(interceptor ErrInterceptor[R]) {
	f.errInterceptor = interceptor
}

func (f *futureImpl[R]) SetUnhandledResultHandler(handler UnhandledResultHandler[R]) {
	f.unhandledResultHandler = handler
}

func (f *futureImpl[R]) Future() Future[R] {
	return f
}
