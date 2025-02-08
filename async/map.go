package async

import (
	"context"
	"github.com/brickingsoft/errors"
)

// Map
// 映射
func Map[SRC any, DST any](ctx context.Context, sf Future[SRC], handle func(ctx context.Context, sfe SRC) (df Future[DST])) (dst Future[DST]) {
	promise, promiseErr := Make[DST](ctx)
	if promiseErr != nil {
		err := errors.New("map failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(promiseErr))
		dst = FailedImmediately[DST](ctx, err)
		return
	}
	sf.OnComplete(func(ctx context.Context, sfe SRC, sfc error) {
		if sfc != nil {
			promise.Fail(sfc)
			return
		}
		df := handle(ctx, sfe)
		df.OnComplete(func(ctx context.Context, dfe DST, dfc error) {
			promise.Complete(dfe, dfc)
		})
	})
	dst = promise.Future()
	return
}
