package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"io"
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
	Stream     bool
	BufferSize int
	Timeout    time.Duration
}

func newFuture[R any](ctx context.Context, submitter rxp.TaskSubmitter, opts FutureOptions) *futureImpl[R] {
	buffer := opts.BufferSize
	if buffer < 1 {
		buffer = 1
	}
	return &futureImpl[R]{
		ctx:            ctx,
		locker:         sync.RWMutex{},
		active:         true,
		stream:         opts.Stream,
		buffer:         buffer,
		ch:             nil,
		chOwned:        false,
		timeout:        opts.Timeout,
		submitter:      submitter,
		handler:        nil,
		errInterceptor: nil,
	}
}

type futureImpl[R any] struct {
	ctx            context.Context
	locker         sync.RWMutex
	active         bool
	stream         bool
	buffer         int
	ch             chan Result[R]
	chOwned        bool
	timeout        time.Duration
	submitter      rxp.TaskSubmitter
	handler        ResultHandler[R]
	errInterceptor ErrInterceptor[R]
}

func (f *futureImpl[R]) resultCh() chan Result[R] {
	if f.ch == nil {
		f.ch = make(chan Result[R], f.buffer)
		f.chOwned = true
	}
	return f.ch
}

func (f *futureImpl[R]) recv(ch <-chan Result[R]) (r Result[R], err error) {
	ctxHasCancel := f.ctx.Done() != nil
	timeout := f.timeout
	if !ctxHasCancel && timeout < 1 {
		v, ok := <-ch
		if ok {
			r = v
		} else {
			err = Canceled
		}
	} else if ctxHasCancel && timeout < 1 {
		select {
		case <-f.ctx.Done():
			err = newUnexpectedContextError(f.ctx)
			break
		case v, ok := <-ch:
			if ok {
				r = v
			} else {
				err = Canceled
			}
			break
		}
	} else {
		timer := time.NewTimer(timeout)
		select {
		case <-f.ctx.Done():
			err = newUnexpectedContextError(f.ctx)
			break
		case deadline := <-timer.C:
			err = newDeadlineExceededError(deadline)
			break
		case v, ok := <-ch:
			if ok {
				r = v
			} else {
				err = Canceled
			}
			break
		}
		timer.Stop()
	}
	return
}

func (f *futureImpl[R]) handleStreamWhenUnexpectedContextErrorOccur(ch chan Result[R]) {
	if f.stream {
		exec, hasExec := rxp.TryFrom(f.ctx)
		if hasExec {
			exec.TryExecute(f.ctx, func() {
				defer func() {
					_ = recover()
				}()
				for {
					r, ok := <-ch
					if !ok {
						break
					}
					val := r.Value()
					var v interface{} = val
					if v == nil {
						continue
					}
					switch closer := v.(type) {
					case io.Closer:
						_ = closer.Close()
						break
					case Closer:
						closer.Close().OnComplete(DiscardVoidHandler)
						break
					default:
						break
					}
				}
			})
		}
	}
}

func (f *futureImpl[R]) handle() {
	ctx := f.ctx
	ch := f.ch
	stream := f.stream
	for {
		r, err := f.recv(ch)
		if err != nil {
			if IsUnexpectedContextFailed(err) {
				// ctx failed then stop future
				f.Cancel()
				f.handleStreamWhenUnexpectedContextErrorOccur(ch)
				err = errors.Join(Canceled, err)
			} else if IsDeadlineExceeded(err) {
				// timeout
				if !stream { // not stream then stop future
					f.Cancel()
					err = errors.Join(Canceled, err)
				}
			} else if IsCanceled(err) {
				f.Cancel()
			}
			if f.errInterceptor != nil {
				f.errInterceptor.Handle(ctx, *(new(R)), err).OnComplete(f.handler)
			} else {
				f.handler(f.ctx, *(new(R)), err)
			}
			if !stream {
				break
			}
			f.locker.RLock()
			if !f.active {
				f.locker.RUnlock()
				break
			}
			f.locker.RUnlock()
			// continue for stream
			continue
		}

		// handle result
		rVal := r.Value()
		rErr := r.Error()
		if f.errInterceptor != nil {
			f.errInterceptor.Handle(ctx, rVal, rErr).OnComplete(f.handler)
		} else {
			f.handler(f.ctx, rVal, rErr)
		}

		// check cancel
		if IsCanceled(rErr) || !stream {
			break
		}
	}
}

func (f *futureImpl[R]) OnComplete(handler ResultHandler[R]) {
	f.locker.Lock()
	defer f.locker.Unlock()
	if handler == nil {
		panic(errors.New("async.Future: handler is nil"))
		return
	}
	if f.handler != nil {
		panic(errors.New("async.Future: handler already set"))
		return
	}
	if f.ch == nil {
		f.ch = make(chan Result[R], f.buffer)
		f.chOwned = true
	}
	f.handler = handler
	if ok := f.submitter.Submit(f.handle); !ok {
		if f.active {
			f.active = false
			if f.chOwned && f.ch != nil {
				close(f.ch)
			}
		}
		err := errors.Join(Canceled, ExecutorsClosed)
		if f.errInterceptor != nil {
			f.errInterceptor.Handle(f.ctx, *(new(R)), err).OnComplete(f.handler)
		} else {
			f.handler(f.ctx, *(new(R)), err)
		}
		return
	}
	return
}

func (f *futureImpl[R]) Complete(r R, err error) (ok bool) {
	f.locker.Lock()
	defer f.locker.Unlock()
	if !f.active {
		return
	}
	ch := f.resultCh()
	if !f.chOwned {
		return
	}
	ch <- result[R]{
		value: r,
		err:   err,
	}
	if !f.stream {
		f.active = false
		close(f.ch)
	}
	ok = true
	return
}

func (f *futureImpl[R]) Succeed(r R) bool {
	return f.Complete(r, nil)
}

func (f *futureImpl[R]) Fail(err error) bool {
	return f.Complete(*(new(R)), err)
}

func (f *futureImpl[R]) Cancel() {
	f.locker.Lock()
	defer f.locker.Unlock()
	if f.active {
		f.active = false
		if f.chOwned && f.ch != nil {
			close(f.ch)
		}
	}
}

func (f *futureImpl[R]) WithErrInterceptor(v ErrInterceptor[R]) Promise[R] {
	f.errInterceptor = v
	return f
}

func (f *futureImpl[R]) SetResultChan(ch chan Result[R]) (err error) {
	f.locker.Lock()
	defer f.locker.Unlock()
	if ch == nil {
		err = errors.New("async.Promise: channel is nil")
		return
	}
	if f.ch != nil {
		err = errors.New("async.Promise: channel already set")
		return
	}
	f.ch = ch
	f.chOwned = false
	return
}

func (f *futureImpl[R]) Future() (future Future[R]) {
	future = f
	return
}
