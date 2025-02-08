package rxp_test

import (
	"context"
	"github.com/brickingsoft/rxp"
	"math/rand"
	"sync/atomic"
	"testing"
)

type randTask struct {
}

func (task *randTask) Handle(ctx context.Context) {
	rand.Float64()
}

func TestExecutors_TryExecute(t *testing.T) {
	tasks := 100
	counter := new(atomic.Int64)
	executors, err := rxp.New()
	if err != nil {
		t.Error(err)
		return
	}
	ctx := executors.Context()
	defer func(executors rxp.Executors) {
		err := executors.Close()
		if err != nil {
			t.Fatal(err)
			return
		}
		t.Log("goroutines", executors.Goroutines())
		t.Log("executed", counter.Load(), counter.Load() == int64(tasks))
	}(executors)
	submitted := 0
	for i := 0; i < tasks; i++ {
		if executors.TryExecute(ctx, &randTask{}) {
			submitted++
		}
	}
	t.Log("submitted", submitted)
	t.Log("goroutines", executors.Goroutines())
}
