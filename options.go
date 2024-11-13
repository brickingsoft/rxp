package rxp

import (
	"fmt"
	"github.com/brickingsoft/rxp/pkg/maxprocs"
	"time"
)

const (
	defaultMaxGoroutines            = 256 * 1024
	defaultMaxGoroutineIdleDuration = 10 * time.Second
)

type Option func(*Options) error

type Options struct {
	MaxprocsOptions          maxprocs.Options
	MaxGoroutines            int
	MaxGoroutineIdleDuration time.Duration
	CloseTimeout             time.Duration
}

func MinGOMAXPROCS(n int) Option {
	return func(o *Options) error {
		if n > 2 {
			o.MaxprocsOptions.MinGOMAXPROCS = n
		}
		return nil
	}
}

func Procs(fn maxprocs.ProcsFunc) Option {
	return func(o *Options) error {
		if fn == nil {
			return fmt.Errorf("rxp: procs function cannot be nil")
		}
		o.MaxprocsOptions.Procs = fn
		return nil
	}
}

func RoundQuotaFunc(fn maxprocs.RoundQuotaFunc) Option {
	return func(o *Options) error {
		if fn == nil {
			return fmt.Errorf("rxp: round quota function cannot be nil")
		}
		o.MaxprocsOptions.RoundQuotaFunc = fn
		return nil
	}
}

func MaxGoroutines(n int) Option {
	return func(o *Options) error {
		if n < 1 {
			n = defaultMaxGoroutines
		}
		o.MaxGoroutines = n
		return nil
	}
}

func MaxGoroutineIdleDuration(d time.Duration) Option {
	return func(o *Options) error {
		if d < 1 {
			d = defaultMaxGoroutineIdleDuration
		}
		o.MaxGoroutineIdleDuration = d
		return nil
	}
}

func WithCloseTimeout(timeout time.Duration) Option {
	return func(o *Options) error {
		if timeout < 1 {
			timeout = 0
		}
		o.CloseTimeout = timeout
		return nil
	}
}
