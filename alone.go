package rxp

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp/pkg/maxprocs"
	"github.com/brickingsoft/rxp/pkg/rate/counter"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"runtime"
	"time"
)

type alone struct {
	running       bool
	maxGoroutines int64
	done          chan struct{}
	locker        spin.Locker
	goroutines    counter.Counter
	closeTimeout  time.Duration
	undo          maxprocs.Undo
}

func (exec *alone) TryExecute(ctx context.Context, task Task) (err error) {
	if task == nil {
		return errors.New("task is nil", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
	}
	exec.locker.Lock()
	defer exec.locker.Unlock()
	if !exec.running {
		return ErrClosed
	}
	if exec.goroutines.Value() >= exec.maxGoroutines {
		return ErrBusy
	}

	exec.goroutines.Incr()
	go func(ctx context.Context, task Task, exec *alone) {
		task.Handle(ctx)
		exec.goroutines.Decr()
	}(ctx, task, exec)

	return nil
}

func (exec *alone) Execute(ctx context.Context, task Task) (err error) {
	if task == nil {
		err = errors.New("task is nil", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}
	times := 10
	for {
		err = exec.TryExecute(ctx, task)
		if err == nil {
			break
		}
		if IsBusy(err) {
			time.Sleep(ns500)
			times--
			if times < 0 {
				times = 10
				runtime.Gosched()
			}
			continue
		}
		break
	}
	return
}

func (exec *alone) Done() <-chan struct{} {
	return exec.done
}

func (exec *alone) Goroutines() int64 {
	return exec.goroutines.Value()
}

func (exec *alone) GoroutinesLeft() int64 {
	return exec.maxGoroutines - exec.goroutines.Value()
}

func (exec *alone) Running() bool {
	exec.locker.Lock()
	defer exec.locker.Unlock()
	return exec.running
}

func (exec *alone) Close() (err error) {
	exec.locker.Lock()
	defer exec.locker.Unlock()
	if !exec.running {
		err = errors.New("executors already closed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}
	exec.running = false

	defer exec.undo()

	close(exec.done)

	ctx := context.TODO()
	if closeTimeout := exec.closeTimeout; closeTimeout > 0 {
		waitCtx, waitCtxCancel := context.WithTimeout(ctx, closeTimeout)
		waitErr := exec.goroutines.WaitDownTo(waitCtx, 0)
		waitCtxCancel()
		if waitErr != nil {
			err = errors.From(ErrCloseFailed, errors.WithWrap(waitErr))
			return
		}
		return
	}
	if waitErr := exec.goroutines.WaitDownTo(ctx, 0); waitErr != nil {
		err = errors.From(ErrCloseFailed, errors.WithWrap(waitErr))
		return
	}
	return
}

func (exec *alone) start() {
	exec.running = true
	exec.done = make(chan struct{}, 1)
}
