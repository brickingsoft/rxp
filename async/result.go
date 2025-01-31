package async

import (
	"context"
	"io"
)

// Void
// 空
type Void struct{}

// Result
// 结果
type Result[E any] interface {
	// Value 值
	Value() E
	// Error 错误
	Error() error
}

type result[E any] struct {
	value E
	err   error
}

func (r result[E]) Value() E {
	return r.value
}

func (r result[E]) Error() error {
	return r.err
}

// ResultHandler
// 结果处理器
//
// ctx: 来自构建 Promise 是的上下文。
//
// result: 来自 Promise.Succeed() 的结果。
//
// err: 来自 Promise.Failed() 的错误，或者 Promise.Cancel() 的结果，也可能是 Context.Err() 等。
type ResultHandler[R any] func(ctx context.Context, result R, err error)

var DiscardVoidHandler ResultHandler[Void] = func(_ context.Context, _ Void, _ error) {}

type ErrInterceptor[R any] interface {
	Handle(ctx context.Context, value R, err error) (future Future[R])
}

type Closer interface {
	Close() (future Future[Void])
}

func tryCloseCloser(v any) {
	if v == nil {
		return
	}
	switch closer := v.(type) {
	case io.Closer:
		_ = closer.Close()
		break
	case Closer:
		closer.Close().OnComplete(DiscardVoidHandler)
		break
	default:
		break
	}
}
