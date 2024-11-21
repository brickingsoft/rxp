package async

import (
	"context"
	"errors"
	"github.com/brickingsoft/rxp/pkg/rate/spin"
	"sync"
	"sync/atomic"
	"time"
)

// JoinStreamFutures
// 把多个流事未来合并成一个流式未来。
// 当所有未来都结束了才会通知一个 EOF 作为整体结束。
// 成员结束包含结束、取消、超时，但这些不会通知到下游。
func JoinStreamFutures[R any](members []Future[R]) Future[R] {
	membersLen := len(members)
	if membersLen == 0 {
		panic("async: join streams failed cause no streams")
		return nil
	}
	for i := 0; i < membersLen; i++ {
		member := members[i]
		stream := IsStreamFuture[R](member)
		if !stream {
			panic("async: join streams failed cause not all are stream promise")
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
				if IsCanceled(cause) || IsEOF(cause) || IsUnexpectedEOF(cause) {
					s.alive.Add(-1)
					if s.alive.Load() == 0 {
						handler(ctx, entry, EOF)
						return
					}
					return
				}
			}
			handler(ctx, entry, cause)
		})
	}
}

// StreamPromises
// 并行流。
//
// 当所有未来都结束了才会通知一个 EOF 作为整体结束。
func StreamPromises[R any](ctx context.Context, size int) (v Promise[R], err error) {
	if size < 1 {
		err = errors.New("async: stream promises size < 1")
		return
	}
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
		s, sErr := MustStreamPromise[R](ctx)
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
	ctx      context.Context
	members  []Promise[R]
	index    int
	size     int
	alive    *atomic.Int64
	canceled bool
	locker   sync.Locker
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

func (ss *streamPromises[R]) Complete(r R, err error) {
	ss.locker.Lock()
	if ss.ko() {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		ss.locker.Unlock()
		return
	}
	ss.next().Complete(r, err)
	ss.locker.Unlock()
}

func (ss *streamPromises[R]) Succeed(r R) {
	ss.locker.Lock()
	if ss.ko() {
		tryCloseResultWhenUnexpectedlyErrorOccur(r)
		ss.locker.Unlock()
		return
	}
	ss.next().Succeed(r)
	ss.locker.Unlock()
}

func (ss *streamPromises[R]) Fail(cause error) {
	ss.locker.Lock()
	if ss.ko() {
		ss.locker.Unlock()
		return
	}
	ss.next().Fail(cause)
	ss.locker.Unlock()
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

func (ss *streamPromises[R]) SetDeadline(_ time.Time) {
	return
}

func (ss *streamPromises[R]) Deadline() (deadline time.Time, ok bool) {
	deadline, ok = ss.ctx.Deadline()
	return
}

func (ss *streamPromises[R]) UnexpectedEOF() (err error) {
	if ctxErr := ss.ctx.Err(); ctxErr != nil {
		err = errors.Join(UnexpectedEOF, ctxErr)
	}
	return
}

func (ss *streamPromises[R]) Future() (future Future[R]) {
	futures := make([]Future[R], 0, len(ss.members))
	for _, m := range ss.members {
		futures = append(futures, m.Future())
	}
	future = &streamFutures[R]{
		members: futures,
		alive:   ss.alive,
	}
	return
}
