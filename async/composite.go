package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"sync"
)

// Composite
// 组合多个许诺
//
// 同时监听未来，并打包结果集。
//
// 注意：每一个结果都是独立的，所以错误也是独立的。
func Composite[R []Result[E], E any](ctx context.Context, promises []Promise[E]) (future Future[R]) {
	promisesLen := len(promises)
	if promisesLen == 0 {
		future = FailedImmediately[R](ctx, errors.New("empty promises", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
		return
	}
	members := make([]Future[E], promisesLen)
	for i, member := range promises {
		if member == nil {
			future = FailedImmediately[R](ctx, errors.New("one of promises is nil", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
			return
		}
		members[i] = member.Future()
	}

	promise, promiseErr := Make[R](ctx, WithWait())
	if promiseErr != nil {
		future = FailedImmediately[R](ctx, promiseErr)
		return
	}

	composite := compositeFuture[R, E]{
		promise: promise,
		members: members,
		size:    promisesLen,
		locker:  spin.New(),
		ch:      make(chan Result[E], promisesLen),
	}

	execErr := rxp.Execute(ctx, composite.compose)
	if execErr != nil {
		promise.Fail(execErr)
		return
	}

	future = &composite
	return
}

type compositeFuture[R []Result[E], E any] struct {
	promise Promise[R]
	members []Future[E]
	size    int
	locker  sync.Locker
	ch      chan Result[E]
}

func (composite *compositeFuture[R, E]) OnComplete(handler ResultHandler[R]) {
	composite.promise.Future().OnComplete(handler)
}

func (composite *compositeFuture[R, E]) compose(_ context.Context) {
	for _, member := range composite.members {
		member.OnComplete(composite.handle)
	}
	rs := make(R, 0, len(composite.members))
	for {
		r, ok := <-composite.ch
		if !ok {
			break
		}
		rs = append(rs, r)
	}
	composite.promise.Succeed(rs)
}

func (composite *compositeFuture[R, E]) handle(_ context.Context, value E, err error) {
	composite.ch <- result[E]{
		value: value,
		err:   err,
	}
	composite.locker.Lock()
	composite.size--
	if composite.size == 0 {
		close(composite.ch)
	}
	composite.locker.Unlock()
}
