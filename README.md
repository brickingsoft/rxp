# rxp
快速执行池，一个 `Go` 的协程池。

## 安装
```shell
go get -u github.com/brickingsoft/rxp
```

## 使用
```go
type Task struct {
}

func (task *Task) Handle(ctx context.Context) {
	// do something
}
```
```go
executors, err := rxp.New()
ctx := context.TODO()
executors.TryExecute(ctx, &Task{})
err := executors.Close()
```

## 异步
注意：其中的上下文必须是`Executors.Context()`或者通过`rxp.With()`后的。
```go
// 尝试构建一个许诺
promise, err := async.Make[int](ctx)
if err != nil {
	// hande err
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
## 压测（并行）
环境
```shell
// goos: windows
// goarch: amd64
// cpu: 13th Gen Intel(R) Core(TM) i5-13600K
```
任务
```go
type RandTask struct {
}

func (task *RandTask) Handle(ctx context.Context) {
    rand.Float64()
}
```
结果

| 项目   | 数量      | ns/op | B/op | allocs/op | failed |
|------|---------|-------|------|-----------|--------|
| 共享模式 | 3035029 | 395.8 | 0    | 0         | 0      |
| 独占模式 | 1486959 | 799.1 | 65   | 1         | 0      |

对比

| 项目                           | 数量      | ns/op | B/op | allocs/op |
|------------------------------|---------|-------|------|-----------|
| rxp                          | 3035029 | 395.8 | 0    | 0         |
| github.com/panjf2000/ants/v2 | 2547340 | 468.8 | 0    | 0         |
| github.com/alitto/pond/v2    | 2619438 | 876.9 | 224  | 7         |
| github.com/alitto/pond       | 2025330 | 558.5 | 224  | 7         |

