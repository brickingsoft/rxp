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

type share struct {
	ctx                            context.Context
	ctxCancel                      context.CancelFunc
	maxGoroutines                  int64
	maxReadyGoroutinesIdleDuration time.Duration
	locker                         sync.Locker
	running                        *atomic.Bool
	ready                          []*submitterImpl
	submitters                     sync.Pool
	goroutines                     *counter.Counter
	closeTimeout                   time.Duration
	stopCh                         chan struct{}
	undo                           maxprocs.Undo
}

func (exec *share) Context() context.Context {
	return exec.ctx
}

func (exec *share) TryExecute(ctx context.Context, task Task) bool {
	if task == nil || !exec.Available() {
		return false
	}
	if submitter := exec.TryGetTaskSubmitter(); submitter != nil {
		submitter.Submit(ctx, task)
	}
	return true
}

func (exec *share) Execute(ctx context.Context, task Task) (err error) {
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
			err = errors.From(ErrClosed)
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

func (exec *share) Goroutines() (n int64) {
	n = exec.goroutines.Value()
	return
}

func (exec *share) Available() bool {
	return exec.running.Load() && exec.goroutines.Value() <= exec.maxGoroutines
}

func (exec *share) Running() bool {
	return exec.running.Load()
}

func (exec *share) TryGetTaskSubmitter() (v TaskSubmitter) {
	if !exec.running.Load() {
		return
	}

	var submitter *submitterImpl
	createExecutor := false

	exec.locker.Lock()

	ready := exec.ready
	n := len(ready) - 1
	if n < 0 {
		if exec.Available() {
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
		submitter = vch.(*submitterImpl)
		go func(exec *share, submitter *submitterImpl) {
			exec.handle(submitter)
			exec.submitters.Put(submitter)
			exec.goroutines.Decr()
		}(exec, submitter)
	}
	v = submitter
	return
}

func (exec *share) Close() (err error) {
	if ok := exec.running.CompareAndSwap(true, false); !ok {
		err = errors.New("executors already closed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}

	defer exec.undo()

	close(exec.stopCh)

	ctx := exec.ctx
	cancel := exec.ctxCancel
	defer cancel()

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
	exec.stopCh = make(chan struct{})
	exec.submitters.New = func() interface{} {
		return &submitterImpl{
			lastUseTime: time.Time{},
			ch:          make(chan taskEntry, 1),
		}
	}
	go func(exec *share) {
		ctx := exec.ctx
		stopCh := exec.stopCh
		var scratch []*submitterImpl
		maxExecutorIdleDuration := exec.maxReadyGoroutinesIdleDuration
		stopped := false
		timer := time.NewTimer(maxExecutorIdleDuration)
		for {
			select {
			case <-ctx.Done():
				stopped = true
				break
			case <-stopCh:
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

func (exec *share) clean(scratch *[]*submitterImpl) {
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

func (exec *share) release(submitter *submitterImpl) (ok bool) {
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

func (exec *share) handle(submitter *submitterImpl) {
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
		ctx := exec.ctx
		task.Handle(ctx)
		if !exec.release(submitter) {
			break
		}
	}
}
