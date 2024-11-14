package async

// Group
// 把多个未来的出口合到一个出口
func Group[R any](members []Future[R]) Future[R] {
	return &group[R]{members: members}
}

type group[R any] struct {
	members []Future[R]
}

func (g *group[R]) OnComplete(handler ResultHandler[R]) {
	for _, m := range g.members {
		m.OnComplete(handler)
	}
}
