package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
)

// Future
// 许诺的未来，注册一个异步非堵塞的结果处理器。
type Future[R any] interface {
	// OnComplete
	// 注册一个结果处理器，它是异步非堵塞的。
	// 注意，这步是必须的且只能调用一次（除非使用 AwaitableFuture）。
	OnComplete(handler ResultHandler[R])
}

type futureImpl[R any] struct {
	ctx                    context.Context
	locker                 spin.Locker
	available              bool
	buffer                 int
	ch                     *channel
	submitter              rxp.TaskSubmitter
	handler                ResultHandler[R]
	errInterceptor         ErrInterceptor[R]
	unhandledResultHandler UnhandledResultHandler[R]
}

func (f *futureImpl[R]) Handle(ctx context.Context) {
	ch := f.ch
	handler := f.handler
	errInterceptor := f.errInterceptor
	for {
		e, err := ch.receive(ctx)
		if err != nil {
			var r R
			if e != nil {
				if rv, ok := e.(R); ok {
					r = rv
				}
			}
			if errInterceptor != nil {
				errInterceptor(ctx, r, err).OnComplete(handler)
			} else {
				handler(ctx, r, err)
			}
			if IsCanceled(err) {
				break
			}
			if ch.isOnce() {
				break
			}
			continue
		}
		rv, ok := e.(R)
		if !ok {
			var r R
			err = errors.Join(errors.From(Canceled), errors.New("type of entry is unexpected", errors.WithMeta("rxp", "async")))
			if errInterceptor != nil {
				errInterceptor(ctx, r, err).OnComplete(handler)
			} else {
				handler(f.ctx, r, err)
			}
			break
		}
		handler(ctx, rv, nil)
		if ch.isOnce() {
			break
		}
	}
	f.locker.Lock()
	f.available = false
	f.ch = nil
	f.locker.Unlock()
	// try unhandled
	if remain := ch.remain(); remain > 0 {
		unhandledResultHandler := f.unhandledResultHandler
		for i := 0; i < remain; i++ {
			msg := ch.get()
			if msg.value != nil && unhandledResultHandler != nil {
				r, ok := msg.value.(R)
				if ok {
					unhandledResultHandler(r)
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

	ctx := f.ctx
	if f.handler != nil {
		var r R
		err := errors.From(
			Canceled,
			errors.WithWrap(errors.New("on complete failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(errors.Define("handler already set")))),
		)
		handler(ctx, r, err)
		return
	}

	if handler == nil {
		handler = func(ctx context.Context, result R, err error) {}
		return
	}

	f.handler = handler
	submitter := f.submitter
	task := f
	submitter.Submit(ctx, task)
	f.submitter = nil
	return
}

func (f *futureImpl[R]) Complete(r R, err error) bool {
	f.locker.Lock()
	if !f.available {
		f.locker.Unlock()
		return false
	}
	if ch := f.ch; ch != nil {
		ch.send(r, err)
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
	if ch := f.ch; ch != nil {
		ch.cancel()
	}
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

func (f *futureImpl[R]) StreamMode() bool {
	return f.buffer > 1
}
