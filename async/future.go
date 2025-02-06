package async

import (
	"context"
	"github.com/brickingsoft/errors"
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
			if f.canReceive() {
				continue
			}
			break
		}
		// handle result
		if r, ok := e.(result[R]); ok {
			rVal := r.Value()
			rErr := r.Error()
			if rErr != nil && f.errInterceptor != nil {
				f.errInterceptor(ctx, rVal, rErr).OnComplete(f.handler)
			} else {
				f.handler(f.ctx, rVal, rErr)
			}
		} else {
			err = errors.Join(errors.From(Canceled, errors.WithMeta(errMetaPkgKey, errMetaPkgVal)), errors.New("type of result is unexpected", errors.WithMeta("rxp", "async")))
			if f.errInterceptor != nil {
				f.errInterceptor(ctx, *(new(R)), err).OnComplete(f.handler)
			} else {
				f.handler(f.ctx, *(new(R)), err)
			}
			f.end()
		}
		if f.canReceive() {
			continue
		}
		break
	}
	// try unhandled
	f.handleUnhandledResult()
	// release
	ch := f.channel
	releaseChannel(ch)
}

func (f *futureImpl[R]) OnComplete(handler ResultHandler[R]) {
	if handler == nil {
		panic(errors.New("handler is nil", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
		return
	}
	if f.handler != nil {
		panic(errors.New("handler already set", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
		return
	}
	ctx := f.ctx
	f.handler = handler
	task := f.handle
	f.submitter.Submit(ctx, task)
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
	f.cancel()
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
