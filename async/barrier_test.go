package async_test

import (
	"context"
	"github.com/brickingsoft/rxp/async"
	"sync"
	"testing"
)

func TestBarrier_Do(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()

	barrier := async.NewBarrier[int]()
	wg := new(sync.WaitGroup)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(ctx context.Context, barrier async.Barrier[int], wg *sync.WaitGroup, i int) {
			barrier.Do(ctx, "key", func(promise async.Promise[int]) {
				promise.Succeed(i)
			}).OnComplete(func(ctx context.Context, entry int, cause error) {
				t.Log(entry, cause)
				wg.Done()
			})
		}(ctx, barrier, wg, i)
	}

	wg.Wait()
}

func TestBarrier_Forget(t *testing.T) {
	ctx, closer := prepare()
	defer func() {
		err := closer()
		if err != nil {
			t.Error(err)
		}
	}()

	barrier := async.NewBarrier[int]()
	wg := new(sync.WaitGroup)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(ctx context.Context, barrier async.Barrier[int], wg *sync.WaitGroup, i int) {
			barrier.Do(ctx, "key", func(promise async.Promise[int]) {
				promise.Succeed(i)
				//barrier.Forget("key")
			}).OnComplete(func(ctx context.Context, entry int, cause error) {
				t.Log(entry, cause)
				barrier.Forget("key")
				wg.Done()
			})
		}(ctx, barrier, wg, i)
	}

	wg.Wait()

}
