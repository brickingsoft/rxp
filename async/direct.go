package async

import (
	"context"
	"github.com/brickingsoft/rxp"
)

var directSubmitterInstance = new(directSubmitter)

type directSubmitter struct {
}

func (submitter *directSubmitter) Submit(ctx context.Context, task rxp.Task) {
	exec, has := rxp.TryFrom(ctx)
	if !has {
		return
	}
	_ = exec.DirectExecute(ctx, task)
	return
}
