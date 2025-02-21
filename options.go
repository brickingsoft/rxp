package rxp

import (
	"errors"
	"fmt"
	"github.com/brickingsoft/rxp/pkg/maxprocs"
	"time"
)

const (
	defaultMaxGoroutines                  = 256 * 1024
	defaultMaxReadyGoroutinesIdleDuration = 10 * time.Second
)

// Option
// 选项函数
type Option func(*Options) error

// Options
// 选项
type Options struct {
	// Mode
	// 模式
	Mode Mode
	// MaxprocsOptions
	// 最大处理器选项 maxprocs.Options
	MaxprocsOptions maxprocs.Options
	// MaxGoroutines
	// 最大协程数
	MaxGoroutines int
	// MaxReadyGoroutinesIdleDuration
	// 准备中协程最大闲置时长
	//
	// 注意：该时长不是一个协程的最大空闲时长，是一组准备中的所有协程的最大空闲时长。
	// 即时某个协程刚转为空闲状态，但是准备组已经空闲超时，则整个超时，进行释放处理。
	MaxReadyGoroutinesIdleDuration time.Duration
	// CloseTimeout
	// 关闭超时时长
	CloseTimeout time.Duration
}

// WithMode
// 设置模式。
func WithMode(mode Mode) Option {
	return func(o *Options) error {
		switch mode {
		case AloneMode:
			o.Mode = AloneMode
			break
		case ShareMode:
			o.Mode = ShareMode
			break
		default:
			return errors.New("invalid mode")
		}
		return nil
	}
}

// WithMinGOMAXPROCS
// 最小 GOMAXPROCS 值，只在 linux 环境下有效。一般用于 docker 容器环境。
func WithMinGOMAXPROCS(n int) Option {
	return func(o *Options) error {
		if n > 2 {
			o.MaxprocsOptions.MinGOMAXPROCS = n
		}
		return nil
	}
}

// WithProcs
// 设置最大 GOMAXPROCS 构建函数。
func WithProcs(fn maxprocs.ProcsFunc) Option {
	return func(o *Options) error {
		if fn == nil {
			return fmt.Errorf("rxp: procs function cannot be nil")
		}
		o.MaxprocsOptions.Procs = fn
		return nil
	}
}

// WithRoundQuotaFunc
// 设置整数配额函数
func WithRoundQuotaFunc(fn maxprocs.RoundQuotaFunc) Option {
	return func(o *Options) error {
		if fn == nil {
			return fmt.Errorf("rxp: round quota function cannot be nil")
		}
		o.MaxprocsOptions.RoundQuotaFunc = fn
		return nil
	}
}

// WithMaxGoroutines
// 设置最大协程数
func WithMaxGoroutines(n int) Option {
	return func(o *Options) error {
		if n < 1 {
			n = defaultMaxGoroutines
		}
		o.MaxGoroutines = n
		return nil
	}
}

// WithMaxReadyGoroutinesIdleDuration
// 设置准备中协程最大闲置时长
func WithMaxReadyGoroutinesIdleDuration(d time.Duration) Option {
	return func(o *Options) error {
		if d < 1 {
			d = defaultMaxReadyGoroutinesIdleDuration
		}
		o.MaxReadyGoroutinesIdleDuration = d
		return nil
	}
}

// WithCloseTimeout
// 设置关闭超时时长
func WithCloseTimeout(timeout time.Duration) Option {
	return func(o *Options) error {
		if timeout < 1 {
			timeout = 0
		}
		o.CloseTimeout = timeout
		return nil
	}
}
