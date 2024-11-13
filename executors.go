package rxp

import (
	"context"
	"errors"
	"fmt"
	"github.com/brickingsoft/rxp/pkg/maxprocs"
	"github.com/brickingsoft/rxp/pkg/rate/counter"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrClosed      = errors.New("rxp: executors were closed")
	ErrCloseFailed = errors.New("rxp: executors close failed")
)

func IsClosed(err error) bool {
	return errors.Is(err, ErrClosed)
}

func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}

func IsTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

const (
	ns500 = 500 * time.Nanosecond
)

type Executors interface {
	TryExecute(ctx context.Context, task Task) (ok bool)
	Execute(ctx context.Context, task Task) (err error)
	Goroutines() (n int64)
	Tasks() (n int64)
	TryGetTaskSubmitter() (submitter TaskSubmitter, has bool)
	ReleaseNotUsedTaskSubmitter(submitter TaskSubmitter)
	Close() (err error)
	CloseGracefully() (err error)
}

func New(options ...Option) Executors {
	opts := Options{
		MaxprocsOptions:          maxprocs.Options{},
		MaxGoroutines:            defaultMaxGoroutines,
		MaxGoroutineIdleDuration: defaultMaxGoroutineIdleDuration,
		CloseTimeout:             0,
	}
	if options != nil {
		for _, option := range options {
			optErr := option(&opts)
			if optErr != nil {
				panic(fmt.Errorf("rxp: new executors failed, %v", optErr))
				return nil
			}
		}
	}
	undo, procsErr := maxprocs.Enable(opts.MaxprocsOptions)
	if procsErr != nil {
		panic(fmt.Errorf("rxp: new executors failed, %v", procsErr))
		return nil
	}
	exec := &executors{
		maxGoroutines:            int64(opts.MaxGoroutines),
		maxGoroutineIdleDuration: opts.MaxGoroutineIdleDuration,
		locker:                   spin.New(),
		running:                  atomic.Bool{},
		ready:                    nil,
		submitters:               sync.Pool{},
		tasks:                    new(atomic.Int64),
		goroutines:               counter.New(),
		stopCh:                   nil,
		stopTimeout:              opts.CloseTimeout,
		undo:                     undo,
	}
	exec.start()
	return exec
}

type executors struct {
	maxGoroutines            int64
	maxGoroutineIdleDuration time.Duration
	locker                   sync.Locker
	running                  atomic.Bool
	ready                    []*submitterImpl
	submitters               sync.Pool
	tasks                    *atomic.Int64
	goroutines               *counter.Counter
	stopCh                   chan struct{}
	stopTimeout              time.Duration
	undo                     maxprocs.Undo
}

func (exec *executors) TryExecute(ctx context.Context, task Task) (ok bool) {
	if task == nil || !exec.running.Load() {
		return false
	}
	submitter := exec.getSubmitter()
	if submitter == nil {
		return false
	}
	select {
	case <-ctx.Done():
		exec.ReleaseNotUsedTaskSubmitter(submitter)
		break
	default:
		submitter.Submit(task)
		ok = true
		break
	}

	return
}

func (exec *executors) Execute(ctx context.Context, task Task) (err error) {
	if task == nil || !exec.running.Load() {
		return
	}
	times := 10
	for {
		ok := exec.TryExecute(ctx, task)
		if ok {
			break
		}
		if err = ctx.Err(); err != nil {
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

func (exec *executors) Goroutines() (n int64) {
	n = exec.goroutines.Value()
	return
}

func (exec *executors) Tasks() (n int64) {
	n = exec.tasks.Load()
	return
}

func (exec *executors) TryGetTaskSubmitter() (submitter TaskSubmitter, has bool) {
	submitter = exec.getSubmitter()
	has = submitter != nil
	return
}

func (exec *executors) ReleaseNotUsedTaskSubmitter(submitter TaskSubmitter) {
	exec.release(submitter.(*submitterImpl))
	return
}

func (exec *executors) Close() (err error) {
	defer exec.undo()
	if exec.stopTimeout > 0 {
		err = exec.CloseGracefully()
		if err != nil {
			err = errors.Join(ErrCloseFailed, err)
		}
		return
	}
	exec.running.Store(false)
	exec.shutdown()
	return
}

func (exec *executors) CloseGracefully() (err error) {
	defer exec.undo()
	exec.running.Store(false)
	exec.shutdown()
	if exec.stopTimeout == 0 {
		err = exec.goroutines.WaitDownTo(context.TODO(), 0)
		if err != nil {
			err = errors.Join(ErrCloseFailed, err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.TODO(), exec.stopTimeout)
	err = exec.goroutines.WaitDownTo(ctx, 0)
	cancel()
	if err != nil {
		err = errors.Join(ErrCloseFailed, err)
	}
	return
}

func (exec *executors) start() {
	exec.running.Store(true)
	exec.stopCh = make(chan struct{})
	exec.submitters.New = func() interface{} {
		return &submitterImpl{
			ch:    make(chan Task, 1),
			tasks: exec.tasks,
		}
	}
	go func(exec *executors) {
		var scratch []*submitterImpl
		maxExecutorIdleDuration := exec.maxGoroutineIdleDuration
		stopped := false
		timer := time.NewTimer(maxExecutorIdleDuration)
		for {
			select {
			case <-exec.stopCh:
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
	}(exec)
}

func (exec *executors) clean(scratch *[]*submitterImpl) {
	if !exec.running.Load() {
		return
	}
	maxExecutorIdleDuration := exec.maxGoroutineIdleDuration
	criticalTime := time.Now().Add(-maxExecutorIdleDuration)
	exec.locker.Lock()
	ready := exec.ready
	n := len(ready)
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
	for iot := range tmp {
		tmp[iot].Submit(nil)
		tmp[iot] = nil
	}
}

func (exec *executors) shutdown() {
	close(exec.stopCh)
	exec.locker.Lock()
	ready := exec.ready
	for i := range ready {
		ready[i].Submit(nil)
		ready[i] = nil
	}
	exec.ready = ready[:0]
	exec.locker.Unlock()
}

func (exec *executors) getSubmitter() *submitterImpl {
	var submitter *submitterImpl
	createExecutor := false
	exec.locker.Lock()
	ready := exec.ready
	n := len(ready) - 1
	if n < 0 {
		if exec.goroutines.Value() < exec.maxGoroutines {
			createExecutor = true
			exec.goroutines.Incr()
		}
	} else {
		submitter = ready[n]
		ready[n] = nil
		exec.ready = ready[:n]
	}
	exec.locker.Unlock()
	if submitter == nil {
		if !createExecutor {
			return nil
		}
		vch := exec.submitters.Get()
		submitter = vch.(*submitterImpl)
		go func(exec *executors) {
			exec.handle(submitter)
			exec.submitters.Put(vch)
		}(exec)
	}
	return submitter
}

func (exec *executors) release(submitter *submitterImpl) bool {
	submitter.lastUseTime = time.Now()
	exec.locker.Lock()
	if !exec.running.Load() {
		exec.locker.Unlock()
		return false
	}
	exec.ready = append(exec.ready, submitter)
	exec.locker.Unlock()
	return true
}

func (exec *executors) handle(wch *submitterImpl) {
	for {
		if wch == nil {
			break
		}
		task, ok := <-wch.ch
		if !ok {
			break
		}
		if task == nil {
			break
		}
		task()
		exec.tasks.Add(-1)
		if !exec.release(wch) {
			break
		}
	}
	exec.locker.Lock()
	exec.goroutines.Decr()
	exec.locker.Unlock()
}
