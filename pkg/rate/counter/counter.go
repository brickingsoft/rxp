package counter

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	ns500 = 500 * time.Nanosecond
)

func New() *Counter {
	return new(Counter)
}

type Counter struct {
	n int64
}

func (c *Counter) Incr() int64 {
	return atomic.AddInt64(&c.n, 1)
}

func (c *Counter) Decr() int64 {
	return atomic.AddInt64(&c.n, -1)
}

func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.n)
}

func (c *Counter) WaitDownTo(ctx context.Context, n int64) (err error) {
	times := 10
	for {
		v := c.Value()
		if v <= n {
			break
		}
		if ctx.Err() != nil {
			err = ctx.Err()
			return
		}
		time.Sleep(ns500)
		times--
		if times < 1 {
			times = 10
			runtime.Gosched()
		}
	}
	return
}
