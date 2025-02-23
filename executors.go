package rxp

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp/pkg/maxprocs"
	"github.com/brickingsoft/rxp/pkg/rate/counter"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ns500 = 500 * time.Nanosecond
)

// Task
// 任务
type Task interface {
	// Handle
	// 执行任务
	Handle(ctx context.Context)
}

type Mode int

const (
	// ShareMode
	// 共享协程模式。
	ShareMode Mode = iota
	// AloneMode
	// 独占协程模式
	AloneMode
)

// Executors
// 执行池
//
// 当 goroutine 在 MaxGoroutineIdleDuration 后没有新任务，则会释放。
type Executors interface {
	// TryExecute
	// 尝试执行一个任务。
	TryExecute(ctx context.Context, task Task) (err error)
	// Execute
	// 执行一个任务，如果 goroutine 已满载，则等待有空闲的。
	//
	// 当 context.Context 有错误或者 Executors.Close，则返回错误。
	Execute(ctx context.Context, task Task) (err error)
	// Done
	// 结束通道。
	Done() <-chan struct{}
	// Goroutines
	// 当前 goroutine 数量
	Goroutines() (n int64)
	// GoroutinesLeft
	// 剩余 goroutine 数量
	GoroutinesLeft() int64
	// Running
	// 是否运行中
	Running() bool
	// Close
	// 优雅关闭
	//
	// 此关会等待正在运行的或提交后的任务结束。
	// 如果需要关闭超时，则使用 WithCloseTimeout 进行设置。
	Close() (err error)
}

// New
// 创建执行池
func New(options ...Option) (Executors, error) {
	opts := Options{
		Mode:                      ShareMode,
		MaxprocsOptions:           maxprocs.Options{},
		MaxGoroutines:             defaultMaxGoroutines,
		MaxGoroutinesIdleDuration: defaultMaxGoroutinesIdleDuration,
		CloseTimeout:              0,
	}
	if options != nil {
		for _, option := range options {
			optErr := option(&opts)
			if optErr != nil {
				return nil, errors.New("new executors failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(optErr))
			}
		}
	}
	undo, procsErr := maxprocs.Enable(opts.MaxprocsOptions)
	if procsErr != nil {
		return nil, errors.New("new executors failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(procsErr))
	}

	switch opts.Mode {
	case ShareMode:
		exec := &share{
			maxGoroutines:             int64(opts.MaxGoroutines),
			maxGoroutinesIdleDuration: opts.MaxGoroutinesIdleDuration,
			locker:                    spin.Locker{},
			running:                   atomic.Bool{},
			idles:                     nil,
			submitters:                sync.Pool{},
			goroutines:                counter.Counter{},
			closeTimeout:              opts.CloseTimeout,
			undo:                      undo,
		}
		exec.start()
		return exec, nil
	case AloneMode:
		exec := &alone{
			maxGoroutines: int64(opts.MaxGoroutines),
			locker:        spin.Locker{},
			running:       false,
			goroutines:    counter.Counter{},
			closeTimeout:  opts.CloseTimeout,
			undo:          undo,
		}
		exec.start()
		return exec, nil
	default:
		undo()
		return nil, errors.New("new executors failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(errors.Define("invalid mode")))
	}
}
