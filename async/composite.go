package async

import (
	"context"
	"errors"
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
		future = FailedImmediately[R](ctx, errors.New("async: empty promises"))
		return
	}
	members := make([]Future[E], promisesLen)
	for i, member := range promises {
		if member == nil {
			future = FailedImmediately[R](ctx, errors.New("async: one of promises is nil"))
			return
		}
		members[i] = member.Future()
	}

	promise, promiseErr := MustPromise[R](ctx)
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

func (composite *compositeFuture[R, E]) compose() {
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

func (composite *compositeFuture[R, E]) handle(_ context.Context, entry E, cause error) {
	composite.ch <- result[E]{
		entry: entry,
		cause: cause,
	}
	composite.locker.Lock()
	composite.size--
	if composite.size == 0 {
		close(composite.ch)
	}
	composite.locker.Unlock()
}
