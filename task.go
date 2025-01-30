package rxp

import (
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
	// Cancel
	// 取消提交任务，未调用 Submit 则需要 Cancel 掉。
	Cancel()
}

type submitterImpl struct {
	lastUseTime time.Time
	ch          chan Task
	exec        *executors
}

func (submitter *submitterImpl) Submit(task Task) (ok bool) {
	if task == nil {
		return
	}
	if submitter.exec.running.Load() {
		submitter.ch <- task
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
	close(submitter.ch)
}
