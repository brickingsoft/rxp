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
type Future[R any] interface {
	// OnComplete
	// 注册一个结果处理器，它是异步非堵塞的。
	// 除了 Promise.Fail 给到的错误外，还有可以有以下错误。
	// EOF 流许诺已 Promise.Cancel 而未来正常结束
	// DeadlineExceeded 已超时
	// UnexpectedEOF ctx错误引发非正常结束
	OnComplete(handler ResultHandler[R])
}

func newFuture[R any](ctx context.Context, submitter rxp.TaskSubmitter, stream bool) *futureImpl[R] {
	grc := getGenericResultChan()
	grc.stream = stream
	return &futureImpl[R]{
		ctx:       ctx,
		end:       new(atomic.Bool),
		grc:       grc,
		submitter: submitter,
		handler:   nil,
	}
}

type futureImpl[R any] struct {
	ctx       context.Context
	end       *atomic.Bool
	grc       *genericResultChan
	submitter rxp.TaskSubmitter
	handler   ResultHandler[R]
}

func (f *futureImpl[R]) handle() {
	grc := f.grc
	stopped := false
	isUnexpectedError := false
	for {
		select {
		case <-f.ctx.Done():
			f.end.Store(true)
			ctxErr := f.ctx.Err()
			if ctxErr != nil {
				f.handler(f.ctx, *(new(R)), errors.Join(UnexpectedEOF, ctxErr))
			} else {
				f.handler(f.ctx, *(new(R)), UnexpectedEOF)
			}
			stopped = true
			isUnexpectedError = true
			grc.CloseByUnexpectedError()
			break
		case <-grc.timer.C:
			f.handler(f.ctx, *(new(R)), DeadlineExceeded)
			// stream future will not break when timeout
			// call cancel to stop when need to cancel
			if !grc.stream {
				f.end.Store(true)
				stopped = true
				break
			}
			break
		case entry := <-grc.entries:
			if entry.cause != nil {
				f.handler(f.ctx, *(new(R)), entry.cause)
				if !grc.stream {
					f.end.Store(true)
					stopped = true
				}
				break
			}
			switch value := entry.value.(type) {
			case genericResultChanCancel:
				f.handler(f.ctx, *(new(R)), EOF)
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
				f.handler(f.ctx, *(new(R)), errors.Join(ResultTypeUnmatched, fmt.Errorf("recv type is %s", reflect.TypeOf(entry).String())))
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
		handler(f.ctx, *(new(R)), context.Canceled)
	}

	return
}

func (f *futureImpl[R]) Complete(r R, err error) {
	if f.end.Load() {
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
	if f.end.Load() {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		return
	}
	if f.grc.IsClosed() {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		return
	}
	f.grc.Send(genericResultChanEntry{
		value: r,
		cause: nil,
	})
	if !f.grc.stream {
		f.grc.Close()
	}
	return
}

func (f *futureImpl[R]) Fail(cause error) {
	if f.end.Load() {
		return
	}
	if f.grc.IsClosed() {
		return
	}
	f.grc.Send(genericResultChanEntry{
		value: *(new(R)),
		cause: cause,
	})
	if !f.grc.stream {
		f.grc.Close()
	}
}

func (f *futureImpl[R]) Cancel() {
	if f.end.Load() {
		return
	}
	f.grc.Close()
}

func (f *futureImpl[R]) SetDeadline(deadline time.Time) {
	if f.end.Load() {
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
