package async

import "errors"

// Awaitable
// 同步等待器
type Awaitable[R any] interface {
	// Await
	// 等待未来结果
	Await() (r R, err error)
}

// Await
// 同步等待未来结果
//
// 注意，非无限流许诺只有一个未来，而无限流许诺可能有多个未来。
//
// 对于无限流许诺，直到 err 不为空时才算结束。
func Await[R any](future Future[R]) (r R, err error) {
	awaitable, ok := future.(Awaitable[R])
	if !ok {
		err = errors.New("async: future is not a Awaitable[R]")
		return
	}
	r, err = awaitable.Await()
	return
}
