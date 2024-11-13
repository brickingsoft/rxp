package rxp

import (
	"context"
)

type contextKey struct{}

func With(ctx context.Context, exec Executors) context.Context {
	return context.WithValue(ctx, contextKey{}, exec)
}

func From(ctx context.Context) Executors {
	exec, ok := ctx.Value(contextKey{}).(Executors)
	if ok && exec != nil {
		return exec
	}
	panic("rxp: there is no executors in context")
	return nil
}

func TryExecute(ctx context.Context, task Task) bool {
	if task == nil {
		return false
	}
	exec := From(ctx)
	return exec.TryExecute(ctx, task)
}

func Execute(ctx context.Context, task Task) (err error) {
	if task == nil {
		return
	}
	exec := From(ctx)
	err = exec.Execute(ctx, task)
	return
}
