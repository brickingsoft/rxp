package async

import (
	"context"
	"errors"
)

// Awaitable
// 同步等待器
type Awaitable[R any] interface {
	// Await
	// 等待未来结果
	Await() (r R, err error)
}

func AwaitableFuture[R any](future Future[R]) (af Awaitable[R]) {
	ch := make(chan result[R], 1)
	var handler ResultHandler[R] = func(ctx context.Context, r R, cause error) {
		if errors.Is(cause, EOF) {
			close(ch)
			return
		}
		ch <- result[R]{
			entry: r,
			cause: cause,
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
	ch     chan result[R]
}

func (af *awaitableFuture[R]) Await() (r R, err error) {
	ar, ok := <-af.ch
	if !ok {
		err = EOF
		return
	}
	r, err = ar.entry, ar.cause
	return
}
