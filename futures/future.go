package futures

import (
	"sync"
)

//////////////////////////
// Interface definition //
//////////////////////////

// A Future represents an asynchronous operation
type Future[T any] interface {
	Await() (T, error)
}

//////////////////////
// Future Utilities //
//////////////////////

type futureStatus uint

const (
	futurePending futureStatus = iota + 1
	futureResolved
	futureRejected
)

//////////////////////////
// Promise Like Futures //
//////////////////////////

type Resolver[T any] func(T)
type Rejector func(error)

// a future that is similar to a javascript promise
type promiseLikeFuture[T any] struct {
	state        futureStatus
	err          error
	value        T
	awaitHandles []struct {
		signal chan<- struct{}
	}
	mutex sync.Mutex
}

func (c *promiseLikeFuture[T]) Await() (T, error) {

	// scope the defer to this function, so we can release the lock before reading the channel
	ch, needToWait := func() (<-chan struct{}, bool) {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		if c.state == futurePending {
			signal := make(chan struct{}, 1)
			c.awaitHandles = append(c.awaitHandles, struct{ signal chan<- struct{} }{
				signal: signal,
			})
			return signal, true
		} else {
			return nil, false
		}
	}()
	if needToWait {
		<-ch
	}

	if c.state == futureResolved {
		return c.value, nil
	} else { // rejected path
		return *new(T), c.err
	}

}

// A future type similar to JavaScript's Promise
// This runs f in a different goroutine, so be aware of potential race conditions
func PromiseLikeFuture[T any](f func(resolve Resolver[T], reject Rejector)) Future[T] {

	future := &promiseLikeFuture[T]{
		state:        futurePending,
		awaitHandles: make([]struct{ signal chan<- struct{} }, 0),
	}

	resolve := func(t T) {
		future.mutex.Lock()
		defer future.mutex.Unlock()
		if future.state != futurePending {
			return // do nothing if already settled
		}
		future.state = futureResolved
		future.value = t
		for _, handle := range future.awaitHandles {
			handle.signal <- struct{}{} // notify active await
		}

	}
	reject := func(e error) {
		future.mutex.Lock()
		defer future.mutex.Unlock()
		if future.state != futurePending {
			return // do nothing if already resolved
		}
		future.state = futureRejected
		future.err = e
		for _, handle := range future.awaitHandles {
			handle.signal <- struct{}{} // notify active await
		}
	}
	go f(resolve, reject)
	return future
}

///////////////////////
// Goroutine Futures //
///////////////////////

// This runs f in a goroutine and returns a future that settles the the result of f
func GoroutineFuture[T any](f func() (T, error)) Future[T] {
	future := &promiseLikeFuture[T]{
		state:        futurePending,
		awaitHandles: make([]struct{ signal chan<- struct{} }, 0),
	}

	go func() {
		res, err := f()
		future.mutex.Lock()
		defer future.mutex.Unlock()
		if future.state != futurePending {
			return // do nothing if already resolved
		}
		future.state = futureRejected
		future.err = err
		future.value = res
		for _, handle := range future.awaitHandles {
			handle.signal <- struct{}{} // notify active await
		}
	}()
	return future
}

/////////////////////
// Settled Futures //
/////////////////////

// a future that is already settled
type settledFuture[T any] struct {
	state futureStatus
	err   error
	value T
}

func (f *settledFuture[T]) Await() (T, error) {
	if f.state == futureResolved {
		return f.value, nil
	} else {
		return *new(T), f.err
	}
}

// Returns a future that resolves to value
func Resolved[T any](value T) Future[T] {
	return &settledFuture[T]{
		state: futureResolved,
		err:   nil,
		value: value,
	}
}

// Returns a future that rejects with err
func Rejected[T any](err error) Future[T] {
	return &settledFuture[T]{
		state: futureResolved,
		err:   err,
		value: *new(T),
	}
}
