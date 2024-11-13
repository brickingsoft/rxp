# rxp
快速执行池，一个 `Go` 的协程池。

## 安装
```shell
go get -u github.com/brickingsoft/rxp
```

## 使用
```go
ctx := context.Background(rxp.MaxGoroutines(64))
executors := rxp.New()
executors.TryExecute(ctx, func() {
	// do something
})
err := executors.CloseGracefully()
```

## 建议
建议通过 `rxp.With()` 函数来把 `Executors` 关联到 `context.Context` 中。

如关联到 `http.Request` 中的 `Context`，这样可以在请求处理中使用 `rxp.TryExecute()` 来执行任务。

同时也可以使用 `async`，这也是 `async` 使用的必要条件。

## 异步
当关联好`context.Context`后，可以使用 `async` 来异步编程。
```go
// 尝试构建一个许诺
promise, ok := async.TryPromise[int](ctx)
if !ok {
    return
}
// 完成许诺，可以在任何地方完成。
promise.Succeed(1)
// 许诺一个未来，一般用于异步的返回。
future := promise.Future()
// 处理未来
future.OnComplete(func(ctx context.Context, entry int, cause error) { 
	// ctx   是和构建时使用的同一个上下文
	// entry 是完成许诺的值。
	// cause 是失败许诺的错误。
})
```
建议用法
```go
func Foo[int](ctx context.Context) (future async.Future[int]) {
	promise, ok := async.TryPromise[int](ctx)
	if !ok {
		future = async.FailedImmediately[int](ctx, errors.New("err"))
		return
	}
	Bar(ctx).OnComplete(func(ctx context.Context, entry string, cause error) {
	    if cause != nil {
			promise.Failed(cause)
			return
		}
		
		// handle entry

		promise.Succeed(1)
		return
	})
}

func Bar[string](ctx context.Context) (future async.Future[string]) { 
	// do something
}
```
## 压测
环境
```shell
// goos: windows
// goarch: amd64
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
```
任务
```go
func RandTask() {
    rand.Float64()
}
```
结果

| 项目                               | 数量      | ns/op | B/op | allocs/op | failed |
|----------------------------------|---------|-------|------|-----------|--------|
| BenchmarkExecutors_TryExecute-20 | 2680070 | 426.4 | 0    | 0         | 0      |
| BenchmarkExecutors_Execute-20    | 2636208 | 429.0 | 0    | 0         | 0      |

对比

| 项目                           | 数量      | ns/op | B/op | allocs/op |
|------------------------------|---------|-------|------|-----------|
| rxp                          | 2680070 | 426.4 | 0    | 0         |
| github.com/panjf2000/ants/v2 | 2547340 | 468.8 | 0    | 0         |
| github.com/alitto/pond/v2    | 2619438 | 876.9 | 224  | 7         |
| github.com/alitto/pond       | 2025330 | 558.5 | 224  | 7         |
