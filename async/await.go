package async

import (
	"context"
)

// Awaitable
// 同步等待器
type Awaitable[R any] interface {
	// Await
	// 等待未来结果
	Await() (r R, err error)
}

func AwaitableFuture[R any](future Future[R]) (af Awaitable[R]) {
	ch := make(chan Result[R], 1)
	var handler ResultHandler[R] = func(ctx context.Context, r R, err error) {
		if IsCanceled(err) {
			close(ch)
			return
		}
		ch <- result[R]{
			value: r,
			err:   err,
		}
	}
	future.OnComplete(handler)
	af = &awaitableFuture[R]{
		future: future,
		ch:     ch,
	}
	return
}

type awaitableFuture[R any] struct {
	future Future[R]
	ch     chan Result[R]
}

func (af *awaitableFuture[R]) Await() (r R, err error) {
	ar, ok := <-af.ch
	if !ok {
		err = Canceled
		return
	}
	r, err = ar.Value(), ar.Error()
	return
}
