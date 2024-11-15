package async

import (
	"context"
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
	// 除了 Promise.Fail 给到的错误外，还有可以有以下错误。
	// context.Canceled 已取消
	// context.DeadlineExceeded 已超时
	// ErrFutureWasClosed 非无限流许诺的不正常关闭
	OnComplete(handler ResultHandler[R])
}

// IsStreamFuture
// 判断是否是流
func IsStreamFuture[T any](future Future[T]) bool {
	if future == nil {
		return false
	}
	stream := future.(*futureImpl[T]).stream
	return stream
}

func newFuture[R any](ctx context.Context, submitter rxp.TaskSubmitter, buf int, stream bool) *futureImpl[R] {
	if buf < 1 {
		buf = 1
	}
	return &futureImpl[R]{
		ctx:             ctx,
		futureCtx:       ctx,
		futureCtxCancel: nil,
		stream:          stream,
		closed:          false,
		locker:          spin.New(),
		rch:             make(chan result[R], buf),
		submitter:       submitter,
	}
}

type futureImpl[R any] struct {
	ctx             context.Context
	futureCtx       context.Context
	futureCtxCancel context.CancelFunc
	stream          bool
	closed          bool
	locker          sync.Locker
	rch             chan result[R]
	submitter       rxp.TaskSubmitter
	handler         ResultHandler[R]
}

func (f *futureImpl[R]) closeRCH() {
	f.locker.Lock()
	if f.closed {
		f.locker.Unlock()
		return
	}
	f.closed = true
	close(f.rch)
	f.locker.Unlock()
	return
}

func (f *futureImpl[R]) handle() {
	futureCtx := f.futureCtx
	rch := f.rch
	stopped := false
	isUnexpectedError := false
	for {
		select {
		case <-f.ctx.Done():
			f.handler(f.ctx, *(new(R)), f.ctx.Err())
			stopped = true
			isUnexpectedError = true
			f.closeRCH()
			break
		case <-futureCtx.Done():
			f.handler(f.ctx, *(new(R)), futureCtx.Err())
			stopped = true
			isUnexpectedError = true
			f.closeRCH()
			break
		case ar, ok := <-rch:
			if !ok {
				f.handler(f.ctx, *(new(R)), context.Canceled)
				stopped = true
				break
			}
			f.handler(f.ctx, ar.entry, ar.cause)
			if !f.stream {
				stopped = true
			}
			break
		}
		if stopped {
			break
		}
	}
	if isUnexpectedError {
		for {
			ar, ok := <-rch
			if !ok {
				break
			}
			if ar.cause == nil {
				tryCloseResultWhenUnexpectedlyErrorOccur(ar.entry)
			}
		}
	}
	if f.futureCtxCancel != nil {
		f.futureCtxCancel()
	}
	f.clean()
}

func (f *futureImpl[R]) clean() {
	f.futureCtx = nil
	f.futureCtxCancel = nil
	f.rch = nil
	f.submitter = nil
	f.handler = nil
}

func (f *futureImpl[R]) OnComplete(handler ResultHandler[R]) {
	f.handler = handler
	f.submitter.Submit(f.handle)
}

func (f *futureImpl[R]) Await() (v R, err error) {
	ch := make(chan result[R], 1)
	var handler ResultHandler[R] = func(ctx context.Context, r R, cause error) {
		ch <- result[R]{
			entry: r,
			cause: cause,
		}
		close(ch)
	}
	f.handler = handler
	f.submitter.Submit(f.handle)
	ar := <-ch
	v = ar.entry
	err = ar.cause
	return
}

func (f *futureImpl[R]) Complete(r R, err error) {
	f.locker.Lock()
	if f.closed {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		f.locker.Unlock()
		return
	}
	f.rch <- result[R]{
		entry: r,
		cause: err,
	}
	if !f.stream {
		f.closed = true
		close(f.rch)
	}
	f.locker.Unlock()
	return
}

func (f *futureImpl[R]) Succeed(r R) {
	f.locker.Lock()
	if f.closed {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		f.locker.Unlock()
		return
	}
	f.rch <- result[R]{
		entry: r,
		cause: nil,
	}
	if !f.stream {
		f.closed = true
		close(f.rch)
	}
	f.locker.Unlock()
	return
}

func (f *futureImpl[R]) Fail(cause error) {
	f.locker.Lock()
	if f.closed {
		f.locker.Unlock()
		return
	}
	f.rch <- result[R]{
		cause: cause,
	}
	if !f.stream {
		f.closed = true
		close(f.rch)
	}
	f.locker.Unlock()
}

func (f *futureImpl[R]) Cancel() {
	if f.futureCtxCancel != nil {
		f.futureCtxCancel()
	}
	f.closeRCH()
}

func (f *futureImpl[R]) SetDeadline(deadline time.Time) {
	f.locker.Lock()
	if f.closed {
		f.locker.Unlock()
		return
	}
	f.futureCtx, f.futureCtxCancel = context.WithDeadline(f.futureCtx, deadline)
	f.locker.Unlock()
}

func (f *futureImpl[R]) Future() (future Future[R]) {
	future = f
	return
}
