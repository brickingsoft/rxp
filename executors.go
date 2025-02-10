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
// 一个 goroutine 池。每个 goroutine 具备一个 TaskSubmitter，通过 TaskSubmitter.Submit 进行任务提交。
//
// 当 goroutine 在 MaxGoroutineIdleDuration 后没有新任务，则会释放。
type Executors interface {
	// Context
	// 根上下文，。
	Context() context.Context
	// TryExecute
	// 尝试执行一个任务，如果 goroutine 已满载，则返回 false。
	TryExecute(ctx context.Context, task Task) (ok bool)
	// Execute
	// 执行一个任务，如果 goroutine 已满载，则等待有空闲的。
	//
	// 当 context.Context 有错误或者 Executors.Close、Executors.CloseGracefully，则返回错误。
	Execute(ctx context.Context, task Task) (err error)
	// Goroutines
	// 当前 goroutine 数量
	Goroutines() (n int64)
	// Available
	// 是否存在剩余 goroutine 或 是否运行中
	Available() bool
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
		Ctx:                            nil,
		Mode:                           ShareMode,
		MaxprocsOptions:                maxprocs.Options{},
		MaxGoroutines:                  defaultMaxGoroutines,
		MaxReadyGoroutinesIdleDuration: defaultMaxReadyGoroutinesIdleDuration,
		CloseTimeout:                   0,
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
	rootCtx := opts.Ctx
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(rootCtx)

	switch opts.Mode {
	case ShareMode:
		exec := &share{
			ctx:                            nil,
			ctxCancel:                      cancel,
			maxGoroutines:                  int64(opts.MaxGoroutines),
			maxReadyGoroutinesIdleDuration: opts.MaxReadyGoroutinesIdleDuration,
			locker:                         spin.New(),
			running:                        new(atomic.Bool),
			ready:                          nil,
			submitters:                     sync.Pool{},
			goroutines:                     counter.New(),
			closeTimeout:                   opts.CloseTimeout,
			undo:                           undo,
		}
		exec.ctx = With(ctx, exec)
		exec.start()
		return exec, nil
	case AloneMode:
		exec := &alone{
			ctx:           nil,
			ctxCancel:     cancel,
			maxGoroutines: int64(opts.MaxGoroutines),
			locker:        spin.New(),
			running:       new(atomic.Bool),
			goroutines:    counter.New(),
			closeTimeout:  opts.CloseTimeout,
			undo:          undo,
		}
		exec.ctx = With(ctx, exec)
		exec.start()
		return exec, nil
	default:
		undo()
		cancel()
		return nil, errors.New("new executors failed", errors.WithMeta(errMetaPkgKey, errMetaPkgVal), errors.WithWrap(errors.Define("invalid mode")))
	}
}
