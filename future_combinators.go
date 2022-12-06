package futures

import (
	"sync"
	"time"
)

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

// Chain takes a Future[T] and returns a future that is mapped
// with mapper, passing any errors that occurred to mapper
func ChainErr[T, O any](future Future[T], mapper func(T, error) (O, error)) Future[O] {
	return GoroutineFuture(func() (O, error) {
		mapped, err := mapper(future.Await())
		if err != nil {
			return *new(O), err
		}
		return mapped, nil
	})
}

// WhenComplete takes a Future[T] and returns a future that is settled
// when the original future is, and executes callback on completion
func WhenComplete[T any](future Future[T], callback func(T, error) error) Future[struct{}] {
	return GoroutineFuture(func() (struct{}, error) {
		res, err := future.Await()
		err = callback(res, err)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
}

// Returns a Future that times out with ErrTimeout if future does not settle before timeout
func Timeout[T any](future Future[T], timeout time.Duration) Future[T] {
	return Race(
		future,
		Chain[struct{}](SleepFuture(timeout), func(s struct{}) (T, error) {
			switch c := future.(type) {
			case CancellableFuture[T]:
				c.Cancel() // cancel the future if its cancellable
			}
			return *new(T), ErrTimeout
		}),
	)
}

////////////////////////
// Future Aggregation //
////////////////////////

// All settles to a slice of results when all the
// futures resolve, or returns an error with any futures that reject
func All[T any](futures ...Future[T]) Future[[]T] {
	return GoroutineFuture(func() ([]T, error) {
		results := make([]T, 0)
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

// Race settles to the first future to finish
func Race[T any](futures ...Future[T]) Future[T] {
	return PromiseLikeFuture(func(resolve Resolver[T], reject Rejector) {
		for _, future := range futures {
			WhenComplete(future, func(t T, err error) error {
				if err != nil {
					reject(err)
				} else {
					resolve(t)
				}
				return nil
			})
		}
	})
}

// Any settles to the first future to resolve, or rejects with an error aggregation if none resolve
func Any[T any](futures ...Future[T]) Future[T] {
	return PromiseLikeFuture(func(resolve Resolver[T], reject Rejector) {
		var wg sync.WaitGroup
		var m sync.Mutex
		errs := make([]error, 0)
		for _, future := range futures {
			wg.Add(1)
			WhenComplete(future, func(t T, err error) error {
				if err != nil {
					m.Lock()
					defer m.Unlock()
					errs = append(errs, err)
					wg.Done()
				} else {
					wg.Done()
					resolve(t)
				}
				return nil
			})
		}
		wg.Wait()
		if len(errs) != 0 {
			reject(errorAggregation(errs))
		}
	})
}
