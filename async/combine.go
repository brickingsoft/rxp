package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"sync/atomic"
)

// Combine
// 合并多个流。
func Combine[R any](ctx context.Context, futures []Future[R]) (future Future[R]) {
	if len(futures) == 0 {
		future = FailedImmediately[R](ctx, errors.New("combine failed", errors.WithMeta(errMetaPkgKey, errMetaPkgKey), errors.WithWrap(errors.Define("empty futures"))))
		return
	}
	for _, future = range futures {
		if future == nil {
			future = FailedImmediately[R](ctx, errors.New("combine failed", errors.WithMeta(errMetaPkgKey, errMetaPkgKey), errors.WithWrap(errors.Define("one of futures is nil"))))
			return
		}
		if mode, ok := future.(interface{ StreamMode() bool }); ok {
			if stream := mode.StreamMode(); !stream {
				future = FailedImmediately[R](ctx, errors.New("combine failed", errors.WithMeta(errMetaPkgKey, errMetaPkgKey), errors.WithWrap(errors.Define("one of futures is not stream mode"))))
				return
			}
		}
	}
	promise, promiseErr := Make[R](ctx, WithStream())
	if promiseErr != nil {
		future = FailedImmediately[R](ctx, promiseErr)
		return
	}
	future = &combined[R]{
		promise: promise,
		futures: futures,
		remains: atomic.Int64{},
	}
	return
}

type combined[R any] struct {
	promise Promise[R]
	futures []Future[R]
	remains atomic.Int64
}

func (c *combined[R]) OnComplete(handler ResultHandler[R]) {
	c.remains.Add(int64(len(c.futures)))
	for _, future := range c.futures {
		future.OnComplete(c.Handle)
	}
	c.promise.Future().OnComplete(handler)
}

func (c *combined[R]) Handle(_ context.Context, value R, err error) {
	if err != nil {
		if IsCanceled(err) {
			if n := c.remains.Add(-1); n == 0 {
				c.promise.Cancel()
			}
			return
		}
		c.promise.Fail(err)
		return
	}
	c.promise.Succeed(value)
	return
}
