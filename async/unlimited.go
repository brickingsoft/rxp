package async

import (
	"context"
	"github.com/brickingsoft/rxp"
)

var unlimitedSubmitterInstance = new(unlimitedSubmitter)

type unlimitedSubmitter struct{}

func (submitter *unlimitedSubmitter) Submit(ctx context.Context, task rxp.Task) {
	exec, has := rxp.TryFrom(ctx)
	if !has {
		return
	}
	_ = exec.UnlimitedExecute(ctx, task)
	return
}
