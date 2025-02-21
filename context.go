package rxp

import (
	"context"
	"github.com/brickingsoft/errors"
)

type contextKey struct{}

// With
// 把 Executors 关联到 context.Context
func With(ctx context.Context, exec Executors) context.Context {
	return context.WithValue(ctx, contextKey{}, exec)
}

// From
// 从 context.Context 获取 Executors
//
// 注意，必须先 With ，否则会 panic 。
func From(ctx context.Context) Executors {
	execs, ok := TryFrom(ctx)
	if !ok {
		panic(errors.New("there is no executors in context", errors.WithMeta(errMetaPkgKey, errMetaPkgVal)))
		return nil
	}
	return execs
}

// TryFrom
// 尝试从 context.Context 获取 Executors
func TryFrom(ctx context.Context) (Executors, bool) {
	value := ctx.Value(contextKey{})
	if value == nil {
		return nil, false
	}
	exec, ok := value.(Executors)
	if ok && exec != nil {
		return exec, true
	}
	return nil, false
}

// TryExecute
// 尝试执行一个任务
//
// 注意，必须先 With 。
func TryExecute(ctx context.Context, task Task) error {
	exec, ok := TryFrom(ctx)
	if !ok {
		return errors.New("there is no executors in context", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
	}
	return exec.TryExecute(ctx, task)
}

// Execute
// 执行一个任务
//
// 注意，必须先 With 。
func Execute(ctx context.Context, task Task) error {
	exec, ok := TryFrom(ctx)
	if !ok {
		return errors.New("there is no executors in context", errors.WithMeta(errMetaPkgKey, errMetaPkgVal))
	}
	return exec.Execute(ctx, task)
}
