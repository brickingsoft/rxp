package spin

import (
	"runtime"
	"sync"
	"sync/atomic"
)

const maxBackoff = 16

func New() sync.Locker {
	return &Locker{
		n: atomic.Int64{},
	}
}

type Locker struct {
	n atomic.Int64
}

func (sl *Locker) Lock() {
	backoff := 1
	for !sl.n.CompareAndSwap(0, 1) {
		for i := 0; i < backoff; i++ {
			runtime.Gosched()
		}
		if backoff < maxBackoff {
			backoff <<= 1
		}
	}
}

func (sl *Locker) Unlock() {
	sl.n.Store(0)
}
