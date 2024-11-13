package async

import "context"

// Compose
// 组合未来
//
// 可以用于解决回调地狱的问题。
func Compose[SRC any, DST any](ctx context.Context, sf Future[SRC], handle func(ctx context.Context, sfe SRC) (df Future[DST])) (dst Future[DST]) {
	promise, promiseErr := MustPromise[DST](ctx)
	if promiseErr != nil {
		dst = FailedImmediately[DST](ctx, promiseErr)
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
