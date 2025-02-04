package async

import (
	"context"
	"github.com/brickingsoft/rxp"
)

var unlimitedSubmitterInstance = new(unlimitedSubmitter)

type unlimitedSubmitter struct{}

func (submitter *unlimitedSubmitter) Submit(ctx context.Context, task rxp.Task) (ok bool) {
	exec, has := rxp.TryFrom(ctx)
	if !has {
		return
	}
	err := exec.UnlimitedExecute(ctx, task)
	ok = err == nil
	return
}

func (submitter *unlimitedSubmitter) Cancel() {
	return
}
