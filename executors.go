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
	// ErrClosed 执行池已关闭
	ErrClosed = errors.New("rxp: executors were closed")
	// ErrCloseFailed 关闭执行池失败（一般是关闭超时引发）
	ErrCloseFailed = errors.New("rxp: executors close failed")
)

// IsClosed
// 是否为 ErrClosed 错误
func IsClosed(err error) bool {
	return errors.Is(err, ErrClosed)
}

const (
	ns500 = 500 * time.Nanosecond
)

// Executors
// 执行池
//
// 一个 goroutine 池。每个 goroutine 具备一个 TaskSubmitter，通过 TaskSubmitter.Submit 进行任务提交。
//
// 当 goroutine 在 MaxGoroutineIdleDuration 后没有新任务，则会释放。
type Executors interface {
	// TryExecute
	// 尝试执行一个任务，如果 goroutine 已满载，则返回 false。
	TryExecute(ctx context.Context, task Task) (ok bool)
	// Execute
	// 执行一个任务，如果 goroutine 已满载，则等待有空闲的。
	//
	// 当 context.Context 有错误或者 Executors.Close、Executors.CloseGracefully，则返回错误。
	Execute(ctx context.Context, task Task) (err error)
	// UnlimitedExecute
	// 执行一个不受限型任务。它不受最大协程数限制，可以突破协程数上限，但会增加协程数导致其它方式会得不到协程而失败或等待可用。
	UnlimitedExecute(ctx context.Context, task Task) (err error)
	// DirectExecute
	// 直接执行。它受最大协程数限制，但不会请求任务提交器，而是直接起一个协程。
	DirectExecute(ctx context.Context, task Task) (err error)
	// Goroutines
	// 当前 goroutine 数量
	Goroutines() (n int64)
	// Available
	// 是否存在剩余 goroutine 或 是否运行中
	Available() bool
	// Running
	// 是否运行中
	Running() bool
	// NotifyClose
	// 通知关闭
	NotifyClose() <-chan struct{}
	// TryGetTaskSubmitter
	// 尝试获取一个 TaskSubmitter
	//
	// 如果 goroutine 已满载，则返回 false。
	// 注意：如果获得后没有提交任务，则需要 TaskSubmitter.Cancel 来释放 TaskSubmitter。
	// 否则对应的 goroutine 不会被释放。
	// 如果提交失败则会自动释放。
	TryGetTaskSubmitter() (submitter TaskSubmitter, has bool)
	// Close
	// 关闭
	//
	// 此关闭不会等待正在运行的或提交后的任务结束。
	// 如果需要等待，则使用 CloseGracefully。
	//
	// 如果在创建时通过 WithCloseTimeout 设定了关闭超时，则会自动使用 CloseGracefully 进行关闭。
	Close() (err error)
	// CloseGracefully
	// 优雅关闭
	//
	// 此关闭会等待正在运行的或提交后的任务结束。
	// 如果在创建时通过 WithCloseTimeout 设定了关闭超时，则在超时后退出关闭过程。
	CloseGracefully() (err error)
}

// New
// 创建执行池
func New(options ...Option) Executors {
	opts := Options{
		MaxprocsOptions:                maxprocs.Options{},
		MaxGoroutines:                  defaultMaxGoroutines,
		MaxReadyGoroutinesIdleDuration: defaultMaxReadyGoroutinesIdleDuration,
		CloseTimeout:                   0,
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
		maxGoroutines:                  int64(opts.MaxGoroutines),
		maxReadyGoroutinesIdleDuration: opts.MaxReadyGoroutinesIdleDuration,
		locker:                         spin.New(),
		running:                        new(atomic.Bool),
		ready:                          nil,
		submitters:                     sync.Pool{},
		goroutines:                     counter.New(),
		stopCh:                         nil,
		stopTimeout:                    opts.CloseTimeout,
		notifyClose:                    nil,
		undo:                           undo,
	}
	exec.start()
	return exec
}

type executors struct {
	maxGoroutines                  int64
	maxReadyGoroutinesIdleDuration time.Duration
	locker                         sync.Locker
	running                        *atomic.Bool
	ready                          []*submitterImpl
	submitters                     sync.Pool
	goroutines                     *counter.Counter
	stopCh                         chan struct{}
	stopTimeout                    time.Duration
	notifyClose                    chan struct{}
	undo                           maxprocs.Undo
}

func (exec *executors) TryExecute(ctx context.Context, task Task) (ok bool) {
	if task == nil || !exec.Available() {
		return false
	}
	submitter, has := exec.TryGetTaskSubmitter()
	if !has {
		return false
	}
	select {
	case <-ctx.Done():
		submitter.Cancel()
		break
	default:
		ok = submitter.Submit(task)
		break
	}
	return
}

func (exec *executors) Execute(ctx context.Context, task Task) (err error) {
	if task == nil {
		err = errors.New("rxp: task is nil")
		return
	}
	if !exec.running.Load() {
		err = ErrClosed
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

func (exec *executors) UnlimitedExecute(ctx context.Context, task Task) (err error) {
	if task == nil {
		err = errors.New("rxp: task is nil")
		return
	}
	if !exec.running.Load() {
		err = ErrClosed
		return
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	exec.goroutines.Incr()
	go func(task Task, exec *executors) {
		task()
		exec.goroutines.Decr()
	}(task, exec)

	return err
}

func (exec *executors) DirectExecute(ctx context.Context, task Task) (err error) {
	if task == nil {
		err = errors.New("rxp: task is nil")
		return
	}
	exec.goroutines.Incr()
	if err = exec.goroutines.WaitDownTo(ctx, exec.maxGoroutines); err != nil {
		exec.goroutines.Decr()
		return err
	}
	go func(task Task, exec *executors) {
		task()
		exec.goroutines.Decr()
	}(task, exec)
	return err
}

func (exec *executors) Goroutines() (n int64) {
	n = exec.goroutines.Value()
	return
}

func (exec *executors) Available() bool {
	return exec.running.Load() && exec.goroutines.Value() < exec.maxGoroutines
}

func (exec *executors) Running() bool {
	return exec.running.Load()
}

func (exec *executors) TryGetTaskSubmitter() (v TaskSubmitter, has bool) {
	var submitter *submitterImpl
	createExecutor := false
	exec.locker.Lock()
	ready := exec.ready
	n := len(ready) - 1
	if n < 0 {
		if exec.Available() {
			createExecutor = true
		}
	} else {
		submitter = ready[n]
		has = true
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
		has = true
		submitter.exec = exec
		exec.goroutines.Incr()
		go func(exec *executors, submitter *submitterImpl) {
			exec.handle(submitter)
			submitter.exec = nil
			exec.submitters.Put(submitter)
			exec.goroutines.Decr()
		}(exec, submitter)
	}
	v = submitter
	return
}

func (exec *executors) Close() (err error) {
	if exec.stopTimeout > 0 {
		err = exec.CloseGracefully()
		if err != nil {
			err = errors.Join(ErrCloseFailed, err)
		}
		return
	}
	defer exec.undo()
	exec.running.Store(false)
	exec.shutdown()
	close(exec.notifyClose)
	return
}

func (exec *executors) CloseGracefully() (err error) {
	defer exec.undo()
	exec.running.Store(false)
	exec.shutdown()
	ctx := context.TODO()
	if exec.stopTimeout == 0 {
		goroutinesErr := exec.goroutines.WaitDownTo(ctx, 0)
		if goroutinesErr != nil {
			err = errors.Join(ErrCloseFailed, goroutinesErr)
		}
		close(exec.notifyClose)
		return
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, exec.stopTimeout)
	err = exec.goroutines.WaitDownTo(ctx, 0)
	if err != nil {
		cancel()
		close(exec.notifyClose)
		err = errors.Join(ErrCloseFailed, err)
		return
	}
	cancel()
	close(exec.notifyClose)
	return
}

func (exec *executors) NotifyClose() <-chan struct{} {
	return exec.notifyClose
}

func (exec *executors) start() {
	exec.running.Store(true)
	exec.stopCh = make(chan struct{})
	exec.notifyClose = make(chan struct{}, 1)
	exec.submitters.New = func() interface{} {
		return &submitterImpl{
			lastUseTime: time.Time{},
			ch:          make(chan Task, 1),
			exec:        nil,
		}
	}
	go func(exec *executors) {
		var scratch []*submitterImpl
		maxExecutorIdleDuration := exec.maxReadyGoroutinesIdleDuration
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
	maxExecutorIdleDuration := exec.maxReadyGoroutinesIdleDuration
	criticalTime := time.Now().Add(-maxExecutorIdleDuration)
	exec.locker.Lock()
	if !exec.running.Load() {
		exec.locker.Unlock()
		return
	}
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
		tmp[iot].stop()
		tmp[iot] = nil
	}
}

func (exec *executors) shutdown() {
	close(exec.stopCh)
	exec.locker.Lock()
	ready := exec.ready
	for i := range ready {
		ready[i].stop()
		ready[i] = nil
	}
	exec.ready = ready[:0]
	exec.locker.Unlock()
}

func (exec *executors) release(submitter *submitterImpl) bool {
	if !exec.running.Load() {
		return false
	}
	exec.locker.Lock()
	submitter.lastUseTime = time.Now()
	exec.ready = append(exec.ready, submitter)
	exec.locker.Unlock()
	return true
}

func (exec *executors) handle(submitter *submitterImpl) {
	for {
		if submitter == nil {
			break
		}
		task, ok := <-submitter.ch
		if !ok {
			break
		}
		if task == nil {
			break
		}
		task()
		if !exec.release(submitter) {
			break
		}
		if !exec.running.Load() {
			break
		}
	}
}
