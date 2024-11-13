package async

import (
	"context"
	"io"
	"reflect"
)

type Void struct{}

type Result[E any] interface {
	Succeed() bool
	Failed() bool
	Entry() E
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

type ResultHandler[E any] func(ctx context.Context, entry E, cause error)

func tryCloseResultWhenUnexpectedlyErrorOccur[R any](ar result[R]) {
	if ar.cause == nil {
		r := ar.entry
		ri := reflect.ValueOf(r).Interface()
		closer, isCloser := ri.(io.Closer)
		if isCloser {
			_ = closer.Close()
		}
	}
}
