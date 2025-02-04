package async

import (
	"context"
	"github.com/brickingsoft/rxp"
)

var directSubmitterInstance = new(directSubmitter)

type directSubmitter struct {
}

func (submitter *directSubmitter) Submit(ctx context.Context, task rxp.Task) (ok bool) {
	exec, has := rxp.TryFrom(ctx)
	if !has {
		return
	}
	err := exec.DirectExecute(ctx, task)
	ok = err == nil
	return
}

func (submitter *directSubmitter) Cancel() {
	return
}
