package async

import (
	"context"
	"github.com/brickingsoft/rxp"
)

type unlimitedSubmitter struct {
	exec rxp.Executors
}

func (submitter *unlimitedSubmitter) Submit(task rxp.Task) (ok bool) {
	err := submitter.exec.UnlimitedExecute(task)
	ok = err == nil
	return
}

// UnlimitedPromise
// 不受 rxp.Executors 最大协程限制的许诺
func UnlimitedPromise[R any](ctx context.Context) (promise Promise[R]) {
	exec := rxp.From(ctx)
	submitter := &unlimitedSubmitter{exec: exec}
	promise = newPromise[R](ctx, submitter)
	return
}

// UnlimitedStreamPromise
// 不受 rxp.Executors 最大协程限制的流式许诺
func UnlimitedStreamPromise[R any](ctx context.Context, buf int) (promise Promise[R]) {
	exec := rxp.From(ctx)
	submitter := &unlimitedSubmitter{exec: exec}
	promise = newStreamPromise[R](ctx, submitter, buf)
	return
}
