package rxp

import (
	"context"
	"sync/atomic"
	"time"
)

// Task
// 任务
type Task func(ctx context.Context)

// TaskSubmitter
// 任务提交器
type TaskSubmitter interface {
	// Submit
	// 提交一个任务
	Submit(ctx context.Context, task Task) (ok bool)
	// Cancel
	// 取消提交任务，未调用 Submit 则需要 Cancel 掉。
	Cancel()
}

type taskEntry struct {
	ctx  context.Context
	task Task
}

type submitterImpl struct {
	closed      atomic.Bool
	lastUseTime time.Time
	ch          chan taskEntry
	exec        *executors
}

func (submitter *submitterImpl) Submit(ctx context.Context, task Task) (ok bool) {
	if task == nil {
		return
	}
	if submitter.exec.running.Load() {
		submitter.ch <- taskEntry{
			ctx:  ctx,
			task: task,
		}
		ok = true
	} else {
		submitter.stop()
	}
	return
}

func (submitter *submitterImpl) Cancel() {
	if !submitter.exec.release(submitter) {
		submitter.stop()
	}
	return
}

func (submitter *submitterImpl) stop() {
	if submitter.closed.CompareAndSwap(false, true) {
		submitter.ch <- taskEntry{}
		close(submitter.ch)
	}
}
