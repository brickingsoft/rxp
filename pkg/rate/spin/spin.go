package spin

import (
	"runtime"
	"sync"
	"sync/atomic"
)

const maxBackoff = 16

func New() sync.Locker {
	return new(spinLock)
}

type spinLock struct {
	n atomic.Int64
}

func (sl *spinLock) Lock() {
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

func (sl *spinLock) Unlock() {
	sl.n.Store(0)
}
