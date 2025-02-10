package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
)

// Reduce
// 回收多个 Future 至一个 Future。 注意：不能回收流。
func Reduce[R []Result[E], E any](ctx context.Context, futures []Future[E]) (reduced Future[R]) {

	futuresLen := len(futures)
	if futuresLen == 0 {
		err := errors.New("reduce failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(errors.Define("futures are empty")))
		reduced = FailedImmediately[R](ctx, err)
		return
	}

	for _, future := range futures {
		if future == nil {
			reduced = FailedImmediately[R](ctx, errors.New("reduce failed", errors.WithMeta(errMetaPkgKey, errMetaPkgKey), errors.WithWrap(errors.Define("one of futures is nil"))))
			return
		}
		if mode, ok := future.(interface{ StreamMode() bool }); ok {
			if stream := mode.StreamMode(); stream {
				reduced = FailedImmediately[R](ctx, errors.New("reduce failed", errors.WithMeta(errMetaPkgKey, errMetaPkgKey), errors.WithWrap(errors.Define("one of futures is stream mode"))))
				return
			}
		}
	}

	promise, promiseErr := Make[R](ctx)
	if promiseErr != nil {
		reduced = FailedImmediately[R](ctx, promiseErr)
		return
	}
	reduced = promise.Future()

	members := make([]Future[E], 0, 1)
	for i := 0; i < futuresLen; i++ {
		if member := futures[i]; member != nil {
			members = append(members, member)
		}
	}
	if len(members) == 0 {
		err := errors.New("reduce failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(errors.Define("futures are empty")))
		promise.Fail(err)
		return
	}

	r := &reducer[R, E]{
		promise: promise,
		members: members,
		remains: len(members),
		locker:  spin.Locker{},
		ch:      make(chan Result[E], futuresLen),
	}

	if rErr := r.Reduce(ctx); rErr != nil {
		err := errors.New("reduce failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(rErr))
		promise.Fail(err)
	}

	return
}

type reducer[R []Result[E], E any] struct {
	promise Promise[R]
	members []Future[E]
	remains int
	locker  spin.Locker
	ch      chan Result[E]
}

func (reduce *reducer[R, E]) Reduce(ctx context.Context) error {
	for _, member := range reduce.members {
		member.OnComplete(reduce.OnComplete)
	}
	return rxp.Execute(ctx, reduce)
}

func (reduce *reducer[R, E]) Handle(_ context.Context) {
	rs := make(R, 0, reduce.remains)
	for {
		r, ok := <-reduce.ch
		if !ok {
			break
		}
		rs = append(rs, r)
	}
	reduce.promise.Succeed(rs)
}

func (reduce *reducer[R, E]) OnComplete(_ context.Context, value E, err error) {
	reduce.locker.Lock()
	reduce.ch <- entry[E]{
		value: value,
		err:   err,
	}
	reduce.remains--
	if reduce.remains == 0 {
		close(reduce.ch)
	}
	reduce.locker.Unlock()
}
