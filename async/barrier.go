package async

import (
	"context"
	"sync"
)

func NewBarrier[R any]() Barrier[R] {
	return &barrier[R]{
		locker:  sync.Mutex{},
		callers: nil,
	}
}

// Barrier
// 栅栏，在第一个 Promise 完成前，之后的 Promise 都会共享 第一个的结果。
type Barrier[R any] interface {
	// Do
	// 执行 fn 并返回一个 Future。
	// 当第一个 Promise 完成后会自动 Forget。
	Do(ctx context.Context, key string, fn func(promise Promise[R]), options ...Option) (future Future[R])
	// Forget
	// 遗忘结果，因会自动 Forget，除非需要手动控制。
	// 当第一个完成前 Forget，则等同于 Promise.Cancel，所以手动 Forget 的结果是不可控的。
	Forget(key string)
}

type barrierCaller[R any] struct {
	key    string
	done   bool
	origin Promise[R]
	shared []Promise[R]
}

type barrier[R any] struct {
	locker  sync.Mutex
	callers map[string]*barrierCaller[R]
}

func (b *barrier[R]) Do(ctx context.Context, key string, fn func(promise Promise[R]), options ...Option) (future Future[R]) {
	b.locker.Lock()
	if b.callers == nil {
		b.callers = make(map[string]*barrierCaller[R])
	}
	caller, has := b.callers[key]
	if !has {
		origin, originErr := Make[R](ctx, options...)
		if originErr != nil {
			b.locker.Unlock()
			future = FailedImmediately[R](ctx, originErr)
			return
		}
		shared, sharedErr := Make[R](ctx, options...)
		if sharedErr != nil {
			b.locker.Unlock()
			origin.Future().OnComplete(func(ctx context.Context, entry R, cause error) {})
			origin.Cancel()
			future = FailedImmediately[R](ctx, sharedErr)
			return
		}
		future = shared.Future()

		caller = &barrierCaller[R]{
			key:    key,
			origin: origin,
			shared: make([]Promise[R], 0, 1),
		}
		caller.shared = append(caller.shared, shared)
		b.callers[key] = caller
		b.doCall(caller)
		b.locker.Unlock()
		fn(origin)
		return
	}
	if caller.done {
		b.locker.Unlock()
		future = b.Do(ctx, key, fn, options...)
		return
	}
	shared, sharedErr := Make[R](ctx, options...)
	if sharedErr != nil {
		b.locker.Unlock()
		future = FailedImmediately[R](ctx, sharedErr)
		return
	}
	future = shared.Future()

	caller.shared = append(caller.shared, shared)

	b.locker.Unlock()
	return
}

func (b *barrier[R]) doCall(caller *barrierCaller[R]) {
	caller.origin.Future().OnComplete(func(ctx context.Context, entry R, cause error) {
		b.locker.Lock()
		defer b.locker.Unlock()
		if caller.done {
			return
		}
		for _, promise := range caller.shared {
			promise.Complete(entry, cause)
		}
		caller.done = true
		delete(b.callers, caller.key)
	})
}

func (b *barrier[R]) Forget(key string) {
	b.locker.Lock()
	caller, has := b.callers[key]
	if !has {
		b.locker.Unlock()
		return
	}
	delete(b.callers, key)
	b.locker.Unlock()
	caller.origin.Cancel()
	return
}
