package rxp

import (
	"sync/atomic"
	"time"
)

type Task func()

type TaskSubmitter interface {
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
