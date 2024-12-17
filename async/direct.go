package async

import (
	"context"
	"github.com/brickingsoft/rxp"
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
