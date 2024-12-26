package async

import (
	"context"
	"errors"
	"fmt"
	"github.com/brickingsoft/rxp"
	"reflect"
	"sync/atomic"
	"time"
)

// Future
// 许诺的未来，注册一个异步非堵塞的结果处理器。
//
// 除了 Promise.Fail 给到的错误外，还有可以有以下错误。
//
// EOF 错误为 Promise.Cancel 引发，一般用于流。
//
// UnexpectedEOF 错误为 context.Context 错误时或自身超时，可以通过 IsDeadlineExceeded 来区分是否为超时，包含上下文超时。
type Future[R any] interface {
	// OnComplete
	// 注册一个结果处理器，它是异步非堵塞的。
	OnComplete(handler ResultHandler[R])
}

func newFuture[R any](ctx context.Context, submitter rxp.TaskSubmitter, stream bool) *futureImpl[R] {
	grc := getGenericResultChan()
	grc.stream = stream
	return &futureImpl[R]{
		ctx:           ctx,
		end:           new(atomic.Bool),
		exec:          rxp.From(ctx),
		grc:           grc,
		unexpectedErr: nil,
		deadline:      time.Time{},
		submitter:     submitter,
		handler:       nil,
	}
}

type futureImpl[R any] struct {
	ctx           context.Context
	end           *atomic.Bool
	exec          rxp.Executors
	grc           *genericResultChan
	unexpectedErr error
	deadline      time.Time
	submitter     rxp.TaskSubmitter
	handler       ResultHandler[R]
}

func (f *futureImpl[R]) handle() {
	exec := f.exec
	ctx := f.ctx
	grc := f.grc
	timer := grc.timer
	stopped := false
	isUnexpectedError := false
	for {
		select {
		case <-exec.NotifyClose():
			f.end.Store(true)
			f.unexpectedErr = ExecutorsClosed
			if grc.stream {
				f.handler(f.ctx, *(new(R)), errors.Join(EOF, UnexpectedEOF, ExecutorsClosed))
			} else {
				f.handler(f.ctx, *(new(R)), errors.Join(Canceled, ExecutorsClosed))
			}
			stopped = true
			isUnexpectedError = true
			grc.CloseByUnexpectedError()
			break
		case <-ctx.Done():
			f.end.Store(true)
			var err error
			deadline, ok := ctx.Deadline()
			if ok {
				err = DeadlineExceeded
				f.deadline = deadline
			} else {
				ctxErr := ctx.Err()
				if ctxErr != nil {
					err = ctxErr
					f.unexpectedErr = ctxErr
				} else {
					err = UnexpectedContextFailed
					f.unexpectedErr = UnexpectedContextFailed
				}
			}
			if grc.stream {
				f.handler(f.ctx, *(new(R)), errors.Join(EOF, UnexpectedEOF, err))
			} else {
				f.handler(f.ctx, *(new(R)), errors.Join(Canceled, err))
			}
			stopped = true
			isUnexpectedError = true
			grc.CloseByUnexpectedError()
			break
		case deadline := <-timer.C:
			f.deadline = deadline
			if grc.stream {
				// stream future will not break when timeout
				f.handler(f.ctx, *(new(R)), DeadlineExceeded)
			} else {
				f.handler(f.ctx, *(new(R)), errors.Join(Canceled, DeadlineExceeded))
				f.end.Store(true)
				stopped = true
				break
			}
			break
		case entry := <-grc.entries:
			if entry.cause != nil {
				if entry.value == nil {
					f.handler(f.ctx, *(new(R)), entry.cause)
				} else {
					r, ok := entry.value.(R)
					if ok {
						f.handler(f.ctx, r, entry.cause)
					} else {
						err := errors.Join(entry.cause, ResultTypeUnmatched, fmt.Errorf("recv type is %s", reflect.TypeOf(entry).String()))
						f.handler(f.ctx, *(new(R)), err)
					}
				}
				if !grc.stream {
					f.end.Store(true)
					stopped = true
				}
				break
			}
			switch value := entry.value.(type) {
			case genericResultChanCancel:
				if grc.stream {
					f.handler(f.ctx, *(new(R)), EOF)
				} else {
					f.handler(f.ctx, *(new(R)), Canceled)
				}
				f.end.Store(true)
				stopped = true
				break
			case R:
				f.handler(f.ctx, value, nil)
				if !grc.stream {
					f.end.Store(true)
					stopped = true
					break
				}
				break
			default:
				err := errors.Join(ResultTypeUnmatched, fmt.Errorf("recv type is %s", reflect.TypeOf(entry).String()))
				f.handler(f.ctx, *(new(R)), err)
				if !grc.stream {
					f.end.Store(true)
					stopped = true
				}
				break
			}
		}
		if stopped {
			break
		}
	}
	if isUnexpectedError {
		grc.entries <- genericResultChanEntry{
			value: genericResultChanCancel{},
			cause: nil,
		}
		stopped = false
		for {
			entry, ok := <-grc.entries
			if !ok {
				break
			}
			switch value := entry.value.(type) {
			case error:
				break
			case genericResultChanCancel:
				stopped = true
				break
			default:
				tryCloseResultWhenUnexpectedlyErrorOccur(value)
			}
			if stopped {
				break
			}
		}
	}
	f.clean()
}

func (f *futureImpl[R]) clean() {
	grc := f.grc
	putGenericResultChan(grc)
	f.ctx = nil
	f.grc = nil
	f.submitter = nil
	f.handler = nil
}

func (f *futureImpl[R]) OnComplete(handler ResultHandler[R]) {
	if f.handler != nil {
		return
	}

	if f.end.Load() {
		f.submitter.Cancel()
		f.clean()
		return
	}

	f.handler = handler
	if ok := f.submitter.Submit(f.handle); !ok {
		f.submitter.Cancel()
		f.clean()
		handler(f.ctx, *(new(R)), errors.Join(Canceled, ExecutorsClosed))
	}
	return
}

func (f *futureImpl[R]) UnexpectedEOF() (err error) {
	err = f.unexpectedErr
	return
}

func (f *futureImpl[R]) Deadline() (deadline time.Time, ok bool) {
	deadline = f.deadline
	ok = !deadline.IsZero()
	return
}

func (f *futureImpl[R]) Complete(r R, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			tryCloseResultWhenUnexpectedlyErrorOccur(r)
		}
		return
	}()
	if f.end.Load() || f.grc == nil {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		return
	}
	if f.grc.IsClosed() {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		return
	}

	f.grc.Send(genericResultChanEntry{
		value: r,
		cause: err,
	})
	if !f.grc.stream {
		f.grc.Close()
	}
	return
}

func (f *futureImpl[R]) Succeed(r R) {
	f.Complete(r, nil)
	return
}

func (f *futureImpl[R]) Fail(cause error) {
	f.Complete(*(new(R)), cause)
}

func (f *futureImpl[R]) Cancel() {
	defer func() {
		_ = recover()
	}()
	if f.end.Load() || f.grc == nil {
		return
	}
	f.grc.Close()
}

func (f *futureImpl[R]) SetDeadline(deadline time.Time) {
	defer func() {
		_ = recover()
	}()
	if f.end.Load() || f.grc == nil {
		return
	}
	if f.grc.IsClosed() {
		return
	}
	f.grc.timer.Reset(time.Until(deadline))
}

func (f *futureImpl[R]) Future() (future Future[R]) {
	future = f
	return
}
