package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"sync/atomic"
)

type directSubmitter struct {
	ctx context.Context
}

func (submitter *directSubmitter) Submit(task rxp.Task) (ok bool) {
	ctx := submitter.ctx
	exec, has := rxp.TryFrom(ctx)
	if !has {
		submitter.ctx = nil
		return
	}
	err := exec.DirectExecute(ctx, task)
	submitter.ctx = nil
	ok = err == nil
	return
}

func (submitter *directSubmitter) Cancel() {
	submitter.ctx = nil
	return
}

// DirectPromise
// 直启协程的许诺
func DirectPromise[R any](ctx context.Context) (promise Promise[R]) {
	submitter := &directSubmitter{ctx: ctx}
	promise = newPromise[R](ctx, submitter)
	return
}

// DirectStreamPromise
// 直启协程的流式许诺
func DirectStreamPromise[R any](ctx context.Context) (promise Promise[R]) {
	submitter := &directSubmitter{ctx: ctx}
	promise = newStreamPromise[R](ctx, submitter)
	return
}

// DirectStreamPromises
// 直启协程的并行流。
//
// 当所有未来都结束了才会通知一个 EOF 作为整体结束。
func DirectStreamPromises[R any](ctx context.Context, size int) (v Promise[R], err error) {
	if size < 1 {
		err = errors.New("async: stream promises size < 1")
		return
	}
	ss := &streamPromises[R]{
		members:  make([]Promise[R], size),
		index:    0,
		size:     size,
		alive:    new(atomic.Int64),
		canceled: false,
		locker:   spin.New(),
	}
	for i := 0; i < size; i++ {
		s := DirectStreamPromise[R](ctx)
		ss.members[i] = s
	}
	ss.alive.Store(int64(size))
	v = ss
	return
}
