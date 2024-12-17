package async

import (
	"context"
	"github.com/brickingsoft/rxp"
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
