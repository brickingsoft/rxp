package rxp

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp/pkg/maxprocs"
	"github.com/brickingsoft/rxp/pkg/rate/counter"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type alone struct {
	ctx           context.Context
	ctxCancel     context.CancelFunc
	maxGoroutines int64
	locker        sync.Locker
	running       *atomic.Bool
	goroutines    *counter.Counter
	closeTimeout  time.Duration
	undo          maxprocs.Undo
}

func (exec *alone) Context() context.Context {
	return exec.ctx
}

func (exec *alone) TryExecute(ctx context.Context, task Task) (ok bool) {
	if task == nil || !exec.Available() {
		return false
	}
	if submitter := exec.TryGetTaskSubmitter(); submitter != nil {
		submitter.Submit(ctx, task)
	}
	return true
}

func (exec *alone) Execute(ctx context.Context, task Task) (err error) {
	if task == nil {
		err = errors.New("task is nil", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}
	times := 10
	for {
		if submitter := exec.TryGetTaskSubmitter(); submitter != nil {
			submitter.Submit(ctx, task)
			break
		}
		if !exec.running.Load() {
			err = ErrClosed
			return
		}
		time.Sleep(ns500)
		times--
		if times < 0 {
			times = 10
			runtime.Gosched()
		}
	}
	return
}

func (exec *alone) Goroutines() int64 {
	return exec.goroutines.Value()
}

func (exec *alone) Available() bool {
	return exec.running.Load() && exec.goroutines.Value() <= exec.maxGoroutines
}

func (exec *alone) Running() bool {
	return exec.running.Load()
}

func (exec *alone) TryGetTaskSubmitter() (submitter TaskSubmitter) {
	if !exec.running.Load() {
		return
	}
	exec.locker.Lock()
	if exec.Available() {
		exec.goroutines.Incr()
		exec.locker.Unlock()
		submitter = exec
	} else {
		exec.locker.Unlock()
	}
	return
}

func (exec *alone) Close() (err error) {
	if ok := exec.running.CompareAndSwap(true, false); !ok {
		err = errors.New("executors already closed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}

	defer exec.undo()

	ctx := exec.ctx
	cancel := exec.ctxCancel
	defer cancel()

	if closeTimeout := exec.closeTimeout; closeTimeout > 0 {
		waitCtx, waitCtxCancel := context.WithTimeout(ctx, closeTimeout)
		waitErr := exec.goroutines.WaitDownTo(waitCtx, 0)
		waitCtxCancel()
		if waitErr != nil {
			err = errors.From(ErrCloseFailed, errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(waitErr))
			return
		}
		return
	}
	if waitErr := exec.goroutines.WaitDownTo(ctx, 0); waitErr != nil {
		err = errors.From(ErrCloseFailed, errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(waitErr))
		return
	}
	return
}

func (exec *alone) Submit(ctx context.Context, task Task) {
	go func(ctx context.Context, task Task, exec *alone) {
		task.Handle(ctx)
		exec.goroutines.Decr()
	}(ctx, task, exec)
	return
}

func (exec *alone) start() {
	exec.running.Store(true)
}
