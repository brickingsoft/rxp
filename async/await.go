package async

import (
	"context"
	"github.com/brickingsoft/errors"
)

// Awaitable
// 同步等待器
type Awaitable[R any] interface {
	// Await
	// 等待未来结果
	Await() (r R, err error)
}

// AwaitableFuture
// 转换为同步等待
func AwaitableFuture[R any](future Future[R]) Awaitable[R] {
	af := &awaitableFuture[R]{
		future: future,
		ch:     make(chan result[R], 1),
	}
	future.OnComplete(af.handle)
	return af
}

type awaitableFuture[R any] struct {
	future Future[R]
	ch     chan result[R]
}

func (af *awaitableFuture[R]) Await() (r R, err error) {
	ar, ok := <-af.ch
	if !ok {
		err = errors.From(Canceled, errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}
	r, err = ar.Value(), ar.Error()
	return
}

func (af *awaitableFuture[R]) handle(_ context.Context, r R, err error) {
	ch := af.ch
	if IsCanceled(err) {
		close(ch)
		return
	}
	ch <- result[R]{
		value: r,
		err:   err,
	}
}
