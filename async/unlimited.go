package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"sync/atomic"
)

type unlimitedSubmitter struct {
	ctx context.Context
}

func (submitter *unlimitedSubmitter) Submit(task rxp.Task) (ok bool) {
	ctx := submitter.ctx
	exec, has := rxp.TryFrom(ctx)
	if !has {
		submitter.ctx = nil
		return
	}
	err := exec.UnlimitedExecute(ctx, task)
	submitter.ctx = nil
	ok = err == nil
	return
}

func (submitter *unlimitedSubmitter) Cancel() {
	submitter.ctx = nil
	return
}

// UnlimitedPromise
// 不受 rxp.Executors 最大协程限制的许诺
func UnlimitedPromise[R any](ctx context.Context) (promise Promise[R]) {
	submitter := &unlimitedSubmitter{ctx: ctx}
	promise = newPromise[R](ctx, submitter)
	return
}

// UnlimitedStreamPromise
// 不受 rxp.Executors 最大协程限制的流式许诺
func UnlimitedStreamPromise[R any](ctx context.Context) (promise Promise[R]) {
	submitter := &unlimitedSubmitter{ctx: ctx}
	promise = newStreamPromise[R](ctx, submitter)
	return
}

// UnlimitedStreamPromises
// 不受 rxp.Executors 最大协程限制的并行流。
//
// 当所有未来都结束了才会通知一个 EOF 作为整体结束。
func UnlimitedStreamPromises[R any](ctx context.Context, size int) (v Promise[R], err error) {
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
		s := UnlimitedStreamPromise[R](ctx)
		ss.members[i] = s
	}
	ss.alive.Store(int64(size))
	v = ss
	return
}
