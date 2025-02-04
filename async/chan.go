package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	timers = sync.Pool{
		New: func() interface{} {
			return time.NewTimer(0)
		},
	}
)

func acquireTimer(timeout time.Duration) *time.Timer {
	timer := timers.Get().(*time.Timer)
	timer.Reset(timeout)
	return timer
}

func releaseTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	timer.Stop()
	timers.Put(timer)
}

var (
	defaultStreamChannelSize = runtime.NumCPU() * 2
)

var (
	channel1    = sync.Pool{New: func() interface{} { return newChannel(1) }}
	channel2    = sync.Pool{New: func() interface{} { return newChannel(2) }}
	channel4    = sync.Pool{New: func() interface{} { return newChannel(4) }}
	channel8    = sync.Pool{New: func() interface{} { return newChannel(8) }}
	channel16   = sync.Pool{New: func() interface{} { return newChannel(16) }}
	channel32   = sync.Pool{New: func() interface{} { return newChannel(32) }}
	channel64   = sync.Pool{New: func() interface{} { return newChannel(64) }}
	channel128  = sync.Pool{New: func() interface{} { return newChannel(128) }}
	channel256  = sync.Pool{New: func() interface{} { return newChannel(256) }}
	channel512  = sync.Pool{New: func() interface{} { return newChannel(512) }}
	channel1024 = sync.Pool{New: func() interface{} { return newChannel(1024) }}
	channel2048 = sync.Pool{New: func() interface{} { return newChannel(2048) }}
	channel4096 = sync.Pool{New: func() interface{} { return newChannel(4096) }}
)

func acquireChannel(size int) *channel {
	if size < 2 {
		return channel1.Get().(*channel)
	}
	size = roundupPow2(size)
	switch size {
	case 2:
		return channel2.Get().(*channel)
	case 4:
		return channel4.Get().(*channel)
	case 8:
		return channel8.Get().(*channel)
	case 16:
		return channel16.Get().(*channel)
	case 32:
		return channel32.Get().(*channel)
	case 64:
		return channel64.Get().(*channel)
	case 128:
		return channel128.Get().(*channel)
	case 256:
		return channel256.Get().(*channel)
	case 512:
		return channel512.Get().(*channel)
	case 1024:
		return channel1024.Get().(*channel)
	case 2048:
		return channel2048.Get().(*channel)
	default:
		return channel4096.Get().(*channel)
	}
}

func releaseChannel(c *channel) {
	// reset
	c.sa.Store(true)
	c.ra.Store(true)
	if c.timeout > 0 {
		c.timeout = 0
	}
	// put
	size := c.size()
	if size < 2 {
		channel1.Put(c)
		return
	}
	switch size {
	case 2:
		channel2.Put(c)
		return
	case 4:
		channel4.Put(c)
		return
	case 8:
		channel8.Put(c)
		return
	case 16:
		channel16.Put(c)
		return
	case 32:
		channel32.Put(c)
		return
	case 64:
		channel64.Put(c)
		return
	case 128:
		channel128.Put(c)
		return
	case 256:
		channel256.Put(c)
		return
	case 512:
		channel512.Put(c)
		return
	case 1024:
		channel1024.Put(c)
		return
	case 2048:
		channel2048.Put(c)
		return
	default:
		channel4096.Put(c)
		return
	}
}

func newChannel(size int) *channel {
	c := &channel{
		ch:      make(chan any, size),
		sa:      atomic.Bool{},
		ra:      atomic.Bool{},
		timeout: 0,
	}
	c.sa.Store(true)
	c.ra.Store(true)
	return c
}

type channel struct {
	ch      chan any
	sa      atomic.Bool
	ra      atomic.Bool
	timeout time.Duration
}

func (c *channel) size() int {
	return cap(c.ch)
}

func (c *channel) remain() int {
	return len(c.ch)
}

func (c *channel) setTimeout(timeout time.Duration) {
	c.timeout = timeout
}

func (c *channel) get() any {
	return <-c.ch
}

func (c *channel) receive(ctx context.Context) (v any, err error) {
	if c.timeout < 1 {
		select {
		case r, ok := <-c.ch:
			if !ok {
				err = Canceled
				break
			}
			v = r
			break
		case <-ctx.Done():
			c.end()
			if exec, has := rxp.TryFrom(ctx); has {
				if exec.Running() {
					err = errors.Join(Canceled, &UnexpectedContextError{ctx.Err(), UnexpectedContextFailed})
				} else {
					err = errors.Join(Canceled, ExecutorsClosed)
				}
			} else {
				err = errors.Join(Canceled, &UnexpectedContextError{ctx.Err(), UnexpectedContextFailed})
			}
			break
		}
	} else {
		timer := acquireTimer(c.timeout)
		select {
		case r, ok := <-c.ch:
			if !ok {
				err = Canceled
				break
			}
			v = r
			break
		case deadline := <-timer.C:
			err = &DeadlineExceededError{Deadline: deadline, Err: DeadlineExceeded}
			if c.size() == 1 {
				err = errors.Join(Canceled, err)
			}
			break
		case <-ctx.Done():
			c.end()
			if exec, has := rxp.TryFrom(ctx); has {
				if exec.Running() {
					err = errors.Join(Canceled, &UnexpectedContextError{ctx.Err(), UnexpectedContextFailed})
				} else {
					err = errors.Join(Canceled, ExecutorsClosed)
				}
			} else {
				err = errors.Join(Canceled, &UnexpectedContextError{ctx.Err(), UnexpectedContextFailed})
			}
			break
		}
		releaseTimer(timer)
	}
	c.tryEnd()
	return
}

func (c *channel) send(v any) (ok bool) {
	if !c.sa.Load() {
		return
	}
	c.ch <- v
	ok = true
	if c.size() == 1 {
		c.disableSend()
	}
	return
}

func (c *channel) disableSend() {
	c.sa.CompareAndSwap(true, false)
}

func (c *channel) disableRecv() {
	c.ra.CompareAndSwap(true, false)
}

func (c *channel) tryEnd() {
	if c.size() == 1 {
		c.end()
	}
}

func (c *channel) end() {
	c.disableSend()
	c.disableRecv()
}
