package async

// IsStreamFuture
// 判断是否是流
func IsStreamFuture[T any](future Future[T]) bool {
	if future == nil {
		return false
	}
	impl := future.(*futureImpl[T])
	stream := impl.stream
	return stream
}

// IsStreamPromise
// 判断是否是流
func IsStreamPromise[T any](promise Promise[T]) bool {
	if promise == nil {
		return false
	}
	impl := promise.(*futureImpl[T])
	stream := impl.stream
	return stream
}
