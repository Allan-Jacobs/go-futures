package futures

func tryCancel[T any](f Future[T]) {
	if v, ok := f.(CancellableFuture[T]); ok {
		v.Cancel()
	}
}

func tryCancelAll[T any](f []Future[T]) {
	for _, future := range f {
		tryCancel(future)
	}
}
