package async

import "context"

// SucceedImmediately
// 立刻正确的未来
func SucceedImmediately[R any](ctx context.Context, r R) (f Future[R]) {
	f = &immediatelyFuture[R]{
		ctx:   ctx,
		r:     r,
		cause: nil,
	}
	return
}

// FailedImmediately
// 立刻错误的未来
func FailedImmediately[R any](ctx context.Context, cause error) (f Future[R]) {
	f = &immediatelyFuture[R]{
		ctx:   ctx,
		r:     *(new(R)),
		cause: cause,
	}
	return
}

type immediatelyFuture[R any] struct {
	ctx   context.Context
	r     R
	cause error
}

func (f *immediatelyFuture[R]) OnComplete(handler ResultHandler[R]) {
	handler(f.ctx, f.r, f.cause)
	return
}

func (f *immediatelyFuture[R]) Await() (r R, err error) {
	r, err = f.r, f.cause
	return
}
