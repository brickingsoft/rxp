package rxp

import (
	"sync/atomic"
	"time"
)

// Task
// 任务
type Task func()

// TaskSubmitter
// 任务提交器
type TaskSubmitter interface {
	// Submit
	// 提交一个任务
	Submit(task Task) (ok bool)
}

type submitterImpl struct {
	lastUseTime time.Time
	ch          chan Task
	running     *atomic.Bool
	tasks       *atomic.Int64
}

func (submitter *submitterImpl) Submit(task Task) (ok bool) {
	if task == nil {
		submitter.ch <- task
		ok = true
		return
	}
	if submitter.running.Load() {
		submitter.ch <- task
		submitter.tasks.Add(1)
		ok = true
	}
	return
}
