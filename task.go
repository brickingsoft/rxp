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
	Submit(task Task)
}

type submitterImpl struct {
	lastUseTime time.Time
	ch          chan Task
	tasks       *atomic.Int64
}

func (submitter *submitterImpl) Submit(task Task) {
	submitter.ch <- task
	if task != nil {
		submitter.tasks.Add(1)
	}
}
