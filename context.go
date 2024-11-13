package rxp

import (
	"context"
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
	exec, ok := ctx.Value(contextKey{}).(Executors)
	if ok && exec != nil {
		return exec
	}
	panic("rxp: there is no executors in context")
	return nil
}

// TryExecute
// 尝试执行一个任务
//
// 注意，必须先 With 。
func TryExecute(ctx context.Context, task Task) bool {
	if task == nil {
		return false
	}
	exec := From(ctx)
	return exec.TryExecute(ctx, task)
}

// Execute
// 执行一个任务
//
// 注意，必须先 With 。
func Execute(ctx context.Context, task Task) (err error) {
	if task == nil {
		return
	}
	exec := From(ctx)
	err = exec.Execute(ctx, task)
	return
}
