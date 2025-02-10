package rxp

import (
	"context"
	"github.com/brickingsoft/errors"
	"time"
)

type TaskSubmitters interface {
	// TryGetTaskSubmitter
	// 尝试获取一个 TaskSubmitter
	//
	// 如果 goroutine 已满载，则返回 false。
	// 注意：如果获得后必须提交任务，否则会 goroutine 会一直占用。
	// 如果提交失败则会自动释放。
	TryGetTaskSubmitter() (submitter TaskSubmitter)
}

// TryGetTaskSubmitter
// 尝试从 context.Context 获取 TaskSubmitter
func TryGetTaskSubmitter(ctx context.Context) (TaskSubmitter, error) {
	value := ctx.Value(contextKey{})
	if value == nil {
		return nil, errors.New("no executors found in context", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
	}
	subs, ok := value.(TaskSubmitters)
	if ok && subs != nil {
		if sub := subs.TryGetTaskSubmitter(); sub != nil {
			return sub, nil
		}
		if exec := value.(Executors); exec != nil {
			if exec.Running() {
				return nil, errors.From(ErrBusy)
			}
			return nil, errors.From(ErrClosed)
		}
		return nil, errors.New("invalid executors in context")
	}
	return nil, errors.New("executors in context is not implement TaskSubmitters", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
}

// Task
// 任务
type Task interface {
	// Handle
	// 执行任务
	Handle(ctx context.Context)
}

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
