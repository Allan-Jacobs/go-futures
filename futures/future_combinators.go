package futures

/////////////////////
// Future Chaining //
/////////////////////

// Chain takes a Future[T] and returns a future that is mapped
// with mapper, propagating any errors that occur
func Chain[T, O any](future Future[T], mapper func(T) (O, error)) Future[O] {
	return GoroutineFuture(func() (O, error) {
		res, err := future.Await()
		if err != nil {
			return *new(O), err
		}
		mapped, err := mapper(res)
		if err != nil {
			return *new(O), err
		}
		return mapped, nil
	})
}

////////////////////////
// Future Aggregation //
////////////////////////

// All settles to a slice of results when all the
// futures resolve, or returns an error with any futures that reject
func All[T any](futures ...Future[T]) Future[[]T] {
	return GoroutineFuture(func() ([]T, error) {
		results := make([]T, len(futures))
		for _, f := range futures {
			res, err := f.Await()
			if err != nil {
				return nil, err
			}
			results = append(results, res)
		}
		return results, nil
	})
}
