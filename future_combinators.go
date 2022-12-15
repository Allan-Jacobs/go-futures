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

// OnSettled takes a Future[T] and executes the callback in another goroutine when it is settled
func OnSettled[T any](future Future[T], callback func(T, error)) {
	go func() {
		res, err := future.Await()
		callback(res, err)
	}()
}

// OnReject takes a Future[T] and executes the callback in another goroutine if the future rejects
func OnReject[T any](future Future[T], callback Rejector) {
	go func() {
		_, err := future.Await()
		if err != nil {
			callback(err)
		}
	}()
}

// OnResolve takes a Future[T] and executes the callback in another goroutine if the future resolves
func OnResolve[T any](future Future[T], callback Resolver[T]) {
	go func() {
		res, err := future.Await()
		if err == nil {
			callback(res)
		}
	}()
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

// Returns a future that resolves the same value as the inner future,
// unless the outer future errors, in which case returned the future errors
func Flatten[T any](future Future[Future[T]]) Future[T] {
	return GoroutineFuture(func() (T, error) {
		inner, err := future.Await()
		if err != nil {
			return *new(T), err
		}
		return inner.Await()
	})
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
			OnSettled(future, func(t T, err error) {
				if err != nil {
					reject(err)
				} else {
					resolve(t)
				}
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
			OnSettled(future, func(t T, err error) {
				if err != nil {
					m.Lock()
					defer m.Unlock()
					errs = append(errs, err)
					wg.Done()
				} else {
					wg.Done()
					resolve(t)
				}
			})
		}
		wg.Wait()
		if len(errs) != 0 {
			reject(errorAggregation(errs))
		}
	})
}

func AllSettled[T any](futures []Future[T]) Future[[]struct {
	val T
	err error
}] {
	return GoroutineFuture(func() ([]struct {
		val T
		err error
	}, error) {
		res := make([]struct {
			val T
			err error
		}, 0)
		for _, future := range futures {
			r, err := future.Await()
			res = append(res, struct {
				val T
				err error
			}{val: r, err: err})
		}
		return res, nil
	})
}
