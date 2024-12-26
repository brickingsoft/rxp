package async

import (
	"context"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"io"
	"reflect"
	"runtime"
	"sync"
	"time"
)

// Void
// 空
type Void struct{}

// Result
// 结果
type Result[E any] interface {
	// Succeed 是否成功
	Succeed() bool
	// Failed 是否错误
	Failed() bool
	// Entry 内容
	Entry() E
	// Cause 错误
	Cause() error
}

type result[E any] struct {
	entry E
	cause error
}

func (r result[E]) Succeed() bool {
	return r.cause == nil
}

func (r result[E]) Failed() bool {
	return r.cause != nil
}

func (r result[E]) Entry() E {
	return r.entry
}

func (r result[E]) Cause() error {
	return r.cause
}

// ResultHandler
// 结果处理器
//
// ctx: 来自构建 Promise 是的上下文。
//
// entry: 来自 Promise.Succeed() 的结果。
//
// err: 来自 Promise.Failed() 的错误，或者 Promise.Cancel() 的结果，也可能是 Context.Err() 。
type ResultHandler[E any] func(ctx context.Context, entry E, cause error)

var DiscardVoidHandler ResultHandler[Void] = func(_ context.Context, _ Void, _ error) {}

type Closer interface {
	Close() (future Future[Void])
}

func tryCloseResultWhenUnexpectedlyErrorOccur(v any) {
	if v == nil {
		return
	}
	ri := reflect.ValueOf(v).Interface()
	closer, isCloser := ri.(io.Closer)
	if isCloser {
		_ = closer.Close()
		return
	}
	asyncCloser, isAsyncCloser := ri.(Closer)
	if isAsyncCloser {
		asyncCloser.Close().OnComplete(DiscardVoidHandler)
		return
	}
}

type genericResultChanCancel struct{}

type genericResultChanEntry struct {
	value any
	cause error
}

type genericResultChan struct {
	entries chan genericResultChanEntry
	timer   *time.Timer
	stream  bool
	closed  bool
	lock    sync.Locker
}

func (grc *genericResultChan) IsClosed() bool {
	grc.lock.Lock()
	defer grc.lock.Unlock()
	return grc.closed
}

func (grc *genericResultChan) Close() {
	grc.lock.Lock()
	if grc.closed {
		grc.lock.Unlock()
		return
	}
	grc.entries <- genericResultChanEntry{
		value: genericResultChanCancel{},
		cause: nil,
	}
	grc.closed = true
	grc.lock.Unlock()
}

func (grc *genericResultChan) CloseByUnexpectedError() {
	grc.lock.Lock()
	if grc.closed {
		grc.lock.Unlock()
		return
	}
	grc.closed = true
	grc.lock.Unlock()
}

func (grc *genericResultChan) Send(entry genericResultChanEntry) {
	grc.lock.Lock()
	if grc.closed {
		tryCloseResultWhenUnexpectedlyErrorOccur(entry)
		grc.lock.Unlock()
		return
	}
	grc.entries <- entry
	if !grc.stream {
		grc.closed = true
	}
	grc.lock.Unlock()
}

func (grc *genericResultChan) SetTimeout(timeout time.Duration) {
	grc.lock.Lock()
	if grc.closed {
		grc.lock.Unlock()
		return
	}
	grc.timer.Reset(timeout)
	grc.lock.Unlock()
	return
}

var (
	genericResultChanEntryBuffer  = runtime.NumCPU() * 2
	genericResultChanTimerTimeout = 500 * time.Millisecond
	genericResultChanPool         = sync.Pool{
		New: func() interface{} {
			grc := &genericResultChan{
				entries: make(chan genericResultChanEntry, genericResultChanEntryBuffer),
				timer:   time.NewTimer(genericResultChanTimerTimeout),
				stream:  false,
				closed:  false,
				lock:    spin.New(),
			}
			grc.timer.Stop()
			return grc
		},
	}
)

func getGenericResultChan() *genericResultChan {
	grc := genericResultChanPool.Get().(*genericResultChan)
	return grc
}

func putGenericResultChan(ch *genericResultChan) {
	if ch == nil {
		return
	}
	ch.stream = false
	ch.closed = false
	ch.timer.Stop()
	genericResultChanPool.Put(ch)
}
