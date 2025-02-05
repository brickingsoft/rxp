package rxp

import (
	"context"
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
	Submit(ctx context.Context, task Task)
}

type taskEntry struct {
	ctx  context.Context
	task Task
}

type submitterImpl struct {
	lastUseTime time.Time
	ch          chan taskEntry
}

func (submitter *submitterImpl) Submit(ctx context.Context, task Task) {
	submitter.ch <- taskEntry{
		ctx:  ctx,
		task: task,
	}
	return
}

func (submitter *submitterImpl) stop() {
	submitter.ch <- taskEntry{task: nil}
}
