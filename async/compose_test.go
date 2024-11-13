package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"strconv"
	"testing"
)

func TestCompose(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()
	sf := cs(ctx, "1")
	cf := async.Compose[string, int](
		ctx,
		sf,
		func(ctx context.Context, entry string) (future async.Future[int]) {
			n, err := strconv.ParseInt(entry, 10, 64)
			if err != nil {
				future = async.FailedImmediately[int](ctx, err)
				return
			}
			future = ci(ctx, int(n))
			return
		},
	)
	cf.OnComplete(func(ctx context.Context, entry int, cause error) {
		if cause != nil {
			t.Error(cause)
			return
		}
		t.Log("entry:", entry)
	})

}

func cs(ctx context.Context, s string) async.Future[string] {
	return async.SucceedImmediately(ctx, s)
}

func ci(ctx context.Context, n int) async.Future[int] {
	return async.SucceedImmediately(ctx, n)

}
