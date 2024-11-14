package async

import (
	"context"
	"io"
	"reflect"
)

// Void
// 空
type Void struct{}

// Result
// 结果
type Result[E any] interface {
	// Succeed 是否成功
	Succeed() bool
	// Failed 是否错误
	Failed() bool
	// Entry 内容
	Entry() E
	// Cause 错误
	Cause() error
}

type result[E any] struct {
	entry E
	cause error
}

func (r result[E]) Succeed() bool {
	return r.cause == nil
}

func (r result[E]) Failed() bool {
	return r.cause != nil
}

func (r result[E]) Entry() E {
	return r.entry
}

func (r result[E]) Cause() error {
	return r.cause
}

// ResultHandler
// 结果处理器
//
// ctx: 来自构建 Promise 是的上下文。
//
// entry: 来自 Promise.Succeed() 的结果。
//
// err: 来自 Promise.Failed() 的错误，或者 Promise.Cancel() 的结果，也可能是 Context.Err() 。
type ResultHandler[E any] func(ctx context.Context, entry E, cause error)

func tryCloseResultWhenUnexpectedlyErrorOccur(v any) {
	if v == nil {
		return
	}
	ri := reflect.ValueOf(v).Interface()
	closer, isCloser := ri.(io.Closer)
	if isCloser {
		_ = closer.Close()
	}
}
