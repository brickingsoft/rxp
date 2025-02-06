package async

import (
	"context"
	"github.com/brickingsoft/errors"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"sync"
	"sync/atomic"
)

// JoinStreamFutures
// 把多个流事未来合并成一个流式未来。
// 当所有未来都结束了才会通知一个 EOF 作为整体结束。
// 成员结束包含结束、取消、超时，但这些不会通知到下游。
func JoinStreamFutures[R any](members []Future[R]) Future[R] {
	membersLen := len(members)
	if membersLen == 0 {
		panic(errors.New("join streams failed cause no streams", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
		return nil
	}
	for i := 0; i < membersLen; i++ {
		member := members[i]
		stream := IsStreamFuture[R](member)
		if !stream {
			panic(errors.New("join streams failed cause not all are stream promise", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
			return nil
		}
	}
	s := &streamFutures[R]{
		members: members,
		alive:   &atomic.Int64{},
	}
	s.alive.Store(int64(membersLen))
	return s
}

type streamFutures[R any] struct {
	members []Future[R]
	alive   *atomic.Int64
}

func (s *streamFutures[R]) OnComplete(handler ResultHandler[R]) {
	for _, m := range s.members {
		m.OnComplete(func(ctx context.Context, entry R, cause error) {
			if cause != nil {
				if IsCanceled(cause) {
					s.alive.Add(-1)
					if s.alive.Load() == 0 {
						handler(ctx, entry, cause)
					}
					return
				}
				handler(ctx, entry, cause)
				return
			}
			handler(ctx, entry, cause)
		})
	}
}

// StreamPromises
// 并行流。
//
// 当所有未来都结束了才会通知一个 Canceled 作为整体结束。
func StreamPromises[R any](ctx context.Context, size int, options ...Option) (v Promise[R], err error) {
	if size < 1 {
		err = errors.New("stream promises size < 1", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
		return
	}
	options = append(options, WithStream(), WithWait())
	ss := &streamPromises[R]{
		ctx:      ctx,
		members:  make([]Promise[R], size),
		index:    0,
		size:     size,
		alive:    new(atomic.Int64),
		canceled: false,
		locker:   spin.New(),
	}
	for i := 0; i < size; i++ {
		s, sErr := Make[R](ctx, options...)
		if sErr != nil {
			err = sErr
			return
		}
		ss.members[i] = s
	}
	ss.alive.Store(int64(size))

	v = ss
	return
}

type streamPromises[R any] struct {
	ctx                    context.Context
	members                []Promise[R]
	index                  int
	size                   int
	alive                  *atomic.Int64
	canceled               bool
	locker                 sync.Locker
	errInterceptor         ErrInterceptor[R]
	unhandledResultHandler UnhandledResultHandler[R]
}

func (ss *streamPromises[R]) ko() bool {
	return ss.canceled
}

func (ss *streamPromises[R]) next() (p Promise[R]) {
	p = ss.members[ss.index]
	ss.index++
	if ss.index == ss.size {
		ss.index = 0
	}
	return
}

func (ss *streamPromises[R]) Complete(r R, err error) (ok bool) {
	ss.locker.Lock()
	if ss.ko() {
		ss.locker.Unlock()
		return false
	}
	for i := 0; i < ss.size; i++ {
		member := ss.next()
		if ok = member.Complete(r, err); ok {
			break
		}
	}
	ss.locker.Unlock()
	return
}

func (ss *streamPromises[R]) Succeed(r R) bool {
	return ss.Complete(r, nil)
}

func (ss *streamPromises[R]) Fail(cause error) bool {
	return ss.Complete(*new(R), cause)
}

func (ss *streamPromises[R]) Cancel() {
	ss.locker.Lock()
	if ss.ko() {
		ss.locker.Unlock()
		return
	}
	for _, m := range ss.members {
		m.Cancel()
	}
	ss.canceled = true
	ss.locker.Unlock()
	return
}

func (ss *streamPromises[R]) SetErrInterceptor(v ErrInterceptor[R]) {
	ss.errInterceptor = v
}

func (ss *streamPromises[R]) SetUnhandledResultHandler(handler UnhandledResultHandler[R]) {
	ss.unhandledResultHandler = handler
}

func (ss *streamPromises[R]) Future() (future Future[R]) {
	futures := make([]Future[R], 0, len(ss.members))
	for _, m := range ss.members {
		if ss.errInterceptor != nil {
			m.SetErrInterceptor(ss.errInterceptor)
		}
		if ss.unhandledResultHandler != nil {
			m.SetUnhandledResultHandler(ss.unhandledResultHandler)
		}
		futures = append(futures, m.Future())
	}
	future = &streamFutures[R]{
		members: futures,
		alive:   ss.alive,
	}
	return
}
