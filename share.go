package rxp

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp/pkg/maxprocs"
	"github.com/brickingsoft/rxp/pkg/rate/counter"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type share struct {
	running                        atomic.Bool
	locker                         spin.Locker
	ready                          []*taskSubmitter
	submitters                     sync.Pool
	goroutines                     counter.Counter
	maxGoroutines                  int64
	maxReadyGoroutinesIdleDuration time.Duration
	closeTimeout                   time.Duration
	done                           chan struct{}
	undo                           maxprocs.Undo
}

func (exec *share) TryExecute(ctx context.Context, task Task) error {
	if task == nil {
		return errors.New("task is nil", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
	}
	if !exec.running.Load() {
		return ErrClosed
	}
	if exec.goroutines.Value() >= exec.maxGoroutines {
		return ErrBusy
	}
	if submitter := exec.tryGetTaskSubmitter(); submitter != nil {
		return submitter.submit(ctx, exec.done, task)
	}
	return ErrBusy
}

func (exec *share) Execute(ctx context.Context, task Task) (err error) {
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

func (exec *share) Done() <-chan struct{} {
	return exec.done
}

func (exec *share) Goroutines() (n int64) {
	n = exec.goroutines.Value()
	return
}

func (exec *share) GoroutinesLeft() int64 {
	return exec.maxGoroutines - exec.goroutines.Value()
}

func (exec *share) Running() bool {
	return exec.running.Load()
}

func (exec *share) tryGetTaskSubmitter() (submitter *taskSubmitter) {
	createExecutor := false

	exec.locker.Lock()

	ready := exec.ready
	n := len(ready) - 1
	if n < 0 {
		if exec.goroutines.Value() < exec.maxGoroutines {
			exec.goroutines.Incr()
			createExecutor = true
		}
	} else {
		submitter = ready[n]
		ready[n] = nil
		exec.ready = ready[:n]
	}
	exec.locker.Unlock()

	if submitter == nil {
		if !createExecutor {
			return
		}
		vch := exec.submitters.Get()
		submitter = vch.(*taskSubmitter)
		go func(exec *share, submitter *taskSubmitter) {
			exec.handle(submitter)
			exec.submitters.Put(submitter)
			exec.goroutines.Decr()
		}(exec, submitter)
	}
	return
}

func (exec *share) Close() (err error) {
	if ok := exec.running.CompareAndSwap(true, false); !ok {
		err = errors.New("executors already closed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}

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

func (exec *share) start() {
	exec.running.Store(true)
	exec.done = make(chan struct{})
	exec.submitters.New = func() interface{} {
		return &taskSubmitter{
			lastUseTime: time.Time{},
			ch:          make(chan taskEntry, 1),
		}
	}
	go func(exec *share) {
		done := exec.done
		var scratch []*taskSubmitter
		maxExecutorIdleDuration := exec.maxReadyGoroutinesIdleDuration
		stopped := false
		timer := time.NewTimer(maxExecutorIdleDuration)
		for {
			select {
			case <-done:
				stopped = true
				break
			case <-timer.C:
				exec.clean(&scratch)
				timer.Reset(maxExecutorIdleDuration)
				break
			}
			if stopped {
				break
			}
		}
		timer.Stop()

		exec.locker.Lock()
		ready := exec.ready
		for i := range ready {
			ready[i].stop()
			ready[i] = nil
		}
		exec.ready = ready[:0]
		exec.locker.Unlock()
	}(exec)
}

func (exec *share) clean(scratch *[]*taskSubmitter) {
	exec.locker.Lock()
	ready := exec.ready
	n := len(ready)
	if n == 0 {
		exec.locker.Unlock()
		return
	}

	maxExecutorIdleDuration := exec.maxReadyGoroutinesIdleDuration
	criticalTime := time.Now().Add(-maxExecutorIdleDuration)

	l, r, mid := 0, n-1, 0
	for l <= r {
		mid = (l + r) / 2
		if criticalTime.After(exec.ready[mid].lastUseTime) {
			l = mid + 1
		} else {
			r = mid - 1
		}
	}
	i := r
	if i == -1 {
		exec.locker.Unlock()
		return
	}
	*scratch = append((*scratch)[:0], ready[:i+1]...)
	m := copy(ready, ready[i+1:])
	for i = m; i < n; i++ {
		ready[i] = nil
	}
	exec.ready = ready[:m]
	exec.locker.Unlock()

	tmp := *scratch
	for j := range tmp {
		tmp[j].stop()
		tmp[j] = nil
	}
}

func (exec *share) release(submitter *taskSubmitter) (ok bool) {
	if exec.running.Load() {
		submitter.lastUseTime = time.Now()
		exec.locker.Lock()
		if ok = exec.running.Load(); ok {
			exec.ready = append(exec.ready, submitter)
		}
		exec.locker.Unlock()
	}
	return
}

func (exec *share) handle(submitter *taskSubmitter) {
	if submitter == nil {
		return
	}
	for {
		entry, ok := <-submitter.ch
		if !ok {
			break
		}
		task := entry.task
		if task == nil {
			break
		}
		ctx := entry.ctx
		task.Handle(ctx)
		if !exec.release(submitter) {
			break
		}
	}
}
