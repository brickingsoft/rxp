package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"sync"
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
	Deadline     time.Time
}

func newFuture[R any](ctx context.Context, submitter rxp.TaskSubmitter, opts FutureOptions) *futureImpl[R] {
	buffer := opts.StreamBuffer
	if buffer < 1 {
		buffer = 1
	}
	ch := acquireChannel(buffer)
	if deadline := opts.Deadline; !deadline.IsZero() {
		ch.setDeadline(deadline)
	}
	f := &futureImpl[R]{
		ch:                     ch,
		ctx:                    ctx,
		locker:                 spin.New(),
		available:              true,
		submitter:              submitter,
		handler:                nil,
		errInterceptor:         nil,
		unhandledResultHandler: nil,
	}
	return f
}

type futureImpl[R any] struct {
	ch                     *channel
	ctx                    context.Context
	locker                 sync.Locker
	available              bool
	submitter              rxp.TaskSubmitter
	handler                ResultHandler[R]
	errInterceptor         ErrInterceptor[R]
	unhandledResultHandler UnhandledResultHandler[R]
}

func (f *futureImpl[R]) handle(ctx context.Context) {
	ch := f.ch
	for {
		e, err := ch.receive(ctx)
		if err != nil {
			if f.errInterceptor != nil {
				f.errInterceptor(ctx, *(new(R)), err).OnComplete(f.handler)
			} else {
				f.handler(f.ctx, *(new(R)), err)
			}
			if IsCanceled(err) {
				break
			}
			continue
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
			if ch.isOnce() {
				break
			}
		} else {
			err = errors.Join(errors.From(Canceled, errors.WithMeta(errMetaPkgKey, errMetaPkgVal)), errors.New("type of result is unexpected", errors.WithMeta("rxp", "async")))
			if f.errInterceptor != nil {
				f.errInterceptor(ctx, *(new(R)), err).OnComplete(f.handler)
			} else {
				f.handler(f.ctx, *(new(R)), err)
			}
			break
		}
	}
	f.locker.Lock()
	f.available = false
	f.locker.Unlock()
	// try unhandled
	if remain := ch.remain(); remain > 0 {
		for i := 0; i < remain; i++ {
			v := ch.get()
			if v != nil && f.unhandledResultHandler != nil {
				r, ok := v.(result[R])
				if ok {
					f.unhandledResultHandler(r.value)
				}
			}
		}
	}
	// release
	releaseChannel(ch)
}

func (f *futureImpl[R]) OnComplete(handler ResultHandler[R]) {
	f.locker.Lock()
	defer f.locker.Unlock()
	if handler == nil {
		panic(errors.New("handler is nil", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
		return
	}
	ctx := f.ctx
	if f.handler != nil {
		handler(ctx, *(new(R)), errors.New("handler already set", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
		return
	}
	f.handler = handler
	task := f.handle
	f.submitter.Submit(ctx, task)
	return
}

func (f *futureImpl[R]) Complete(r R, err error) bool {
	f.locker.Lock()
	if !f.available {
		f.locker.Unlock()
		return false
	}
	if ch := f.ch; ch != nil {
		ch.send(result[R]{value: r, err: err})
		if ch.isOnce() {
			f.available = false
		}
		f.locker.Unlock()
		return true
	}
	f.locker.Unlock()
	return false
}

func (f *futureImpl[R]) Succeed(r R) bool {
	return f.Complete(r, nil)
}

func (f *futureImpl[R]) Fail(err error) bool {
	return f.Complete(*(new(R)), err)
}

func (f *futureImpl[R]) Cancel() {
	f.locker.Lock()
	if !f.available {
		f.locker.Unlock()
		return
	}
	ch := f.ch
	ch.cancel()
	f.available = false
	f.locker.Unlock()
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
