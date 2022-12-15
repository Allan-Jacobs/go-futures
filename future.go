package futures

import (
	"sync"
	"time"
)

//////////////////////////
// Interface definition //
//////////////////////////

// A Future[T] represents an asynchronous operation.
// The Future[T] implementations in this module follow a "push" model.
// This means that futures immediately start running their operations
// as soon as they are created. [Future[T].Await] only blocks until the
// future is settled and may be called multiple times across different
// goroutines.
//
// The future follows a basic state machine. The future starts as pending,
// and after it "settles" or completes, it ends as either resolved or
// rejected. Resolved futures return a nil error when awaited,
// and indicate success. Rejected futures return a non-nil error when
// awaited, which is a failure that should be handled.
//
// Example:
//
//	package main
//
//	import (
//		"fmt"
//		"log"
//		"net/http"
//
//		"github.com/Allan-Jacobs/go-futures/futures"
//	)
//
//	// GetAsync wraps [http.Get] in a future
//	func GetAsync(url string) futures.Future[*http.Response] {
//		return futures.GoroutineFuture(func() (*http.Response, error) {
//			return http.Get(url)
//		})
//	}
//
//	func main() {
//		// run GetAsync concurrently
//		results, err := futures.All(
//			GetAsync("https://go.dev"),
//			GetAsync("https://pkg.go.dev"),
//		).Await()
//
//		if err != nil { // evaluates to true when the future rejects
//			log.Fatal("Error: ", err.Error())
//		}
//
//		for _, res := range results {
//			fmt.Printf("Got response from %s %s\n", res.Request.URL, res.Status)
//		}
//	}
type Future[T any] interface {
	// Await blocks the current goroutine until the future is settled.
	// Await may be called by different goroutines concurrently, and it may be called multiple times.
	Await() (T, error)
}

// A future that can be cancelled
type CancellableFuture[T any] interface {
	Future[T]
	// Cancel attempts to cancel the future, and returns true if it is cancelled.
	// Cancel will return false if the future has already completed.
	Cancel() bool
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

// a future that is similar to a javascript promise
type threadsafeFuture[T any] struct {
	state        futureStatus
	err          error
	value        T
	awaitHandles []struct {
		signal chan<- struct{}
	}
	mutex sync.Mutex
}

// a future that wraps threadsafeFuture into a cancellable future
type cancellableThreadsafeFuture[T any] struct {
	threadsafeFuture threadsafeFuture[T]
	signal           chan struct{}
}

func (c *cancellableThreadsafeFuture[T]) Await() (T, error) {
	return c.threadsafeFuture.Await()
}

func (c *cancellableThreadsafeFuture[T]) Cancel() bool {
	c.threadsafeFuture.mutex.Lock()
	defer c.threadsafeFuture.mutex.Unlock()
	if c.threadsafeFuture.state != futurePending {
		// already completed
		return false
	} else {
		c.threadsafeFuture.err = ErrCancelled
		c.threadsafeFuture.state = futureRejected
	}
	for _, handle := range c.threadsafeFuture.awaitHandles {
		handle.signal <- struct{}{} // notify active await
		close(handle.signal)
	}
	c.signal <- struct{}{} // notify operation that it is cancelled
	close(c.signal)
	return true
}

//////////////////////////
// Promise Like Futures //
//////////////////////////

type Resolver[T any] func(T)
type Rejector func(error)

func (c *threadsafeFuture[T]) Await() (T, error) {

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
// This runs f in a different goroutine, so be aware of potential race conditions.
// if resolve or reject are called multiple times, they are no-ops. However multiple
// calls are discouraged, as it acquires a mutex every call.
func PromiseLikeFuture[T any](f func(resolve Resolver[T], reject Rejector)) Future[T] {

	future := &threadsafeFuture[T]{
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
			close(handle.signal)
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
			close(handle.signal)
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
	future := &threadsafeFuture[T]{
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
		if err != nil {
			future.state = futureRejected
		} else {
			future.state = futureResolved
		}

		future.err = err
		future.value = res
		for _, handle := range future.awaitHandles {
			handle.signal <- struct{}{} // notify active await
			close(handle.signal)
		}
	}()
	return future
}

// This runs f in a goroutine and returns a future that settles the the result of f.
// The goroutine should cancel its operation when it recives a value on signal
// This future can be cancelled.
func CancellableGoroutineFuture[T any](f func(signal <-chan struct{}) (T, error)) CancellableFuture[T] {
	signal := make(chan struct{})
	future := &cancellableThreadsafeFuture[T]{
		threadsafeFuture: threadsafeFuture[T]{
			state:        futurePending,
			awaitHandles: make([]struct{ signal chan<- struct{} }, 0),
		},
		signal: signal,
	}

	go func() {
		res, err := f(signal)
		future.threadsafeFuture.mutex.Lock()
		defer future.threadsafeFuture.mutex.Unlock()
		if future.threadsafeFuture.state != futurePending {
			return // do nothing if already resolved
		}
		if err != nil {
			future.threadsafeFuture.state = futureRejected
		} else {
			future.threadsafeFuture.state = futureResolved
		}

		future.threadsafeFuture.err = err
		future.threadsafeFuture.value = res
		for _, handle := range future.threadsafeFuture.awaitHandles {
			handle.signal <- struct{}{} // notify active await
			close(handle.signal)
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
func ResolvedFuture[T any](value T) Future[T] {
	return &settledFuture[T]{
		state: futureResolved,
		err:   nil,
		value: value,
	}
}

// Returns a future that rejects with err
func RejectedFuture[T any](err error) Future[T] {
	return &settledFuture[T]{
		state: futureRejected,
		err:   err,
		value: *new(T),
	}
}

/////////////////////
// Channel Futures //
/////////////////////

// A future that resolves to the next value of the channel
func ChannelFuture[T any](ch <-chan T) Future[T] {
	if ch == nil {
		return RejectedFuture[T](ErrReadFromNilChannel)
	}
	return GoroutineFuture(func() (T, error) {
		res, isOpen := <-ch
		if !isOpen {
			return *new(T), ErrReadFromClosedChannel
		}
		return res, nil
	})
}

///////////////////////////
// Wait Duration Futures //
///////////////////////////

// A future that resolves after the duration.
// This future may take longer than duration,
// but is guaranteed to take at least duration.
func SleepFuture(duration time.Duration) CancellableFuture[struct{}] {
	return CancellableGoroutineFuture(func(signal <-chan struct{}) (struct{}, error) {
		timer := time.NewTimer(duration)
		select {
		case _, isOpen := <-timer.C:
			if !isOpen {
				return struct{}{}, ErrReadFromClosedChannel
			}
			return struct{}{}, nil
		case <-signal:
			timer.Stop()
		}

		return struct{}{}, nil
	})
}

//////////////////////////
// Completeable Futures //
//////////////////////////

type Completer[T any] struct {
	inner *threadsafeFuture[T]
}

func (c Completer[T]) Complete(val T) {
	c.inner.mutex.Lock()
	defer c.inner.mutex.Unlock()
	if c.inner.state == futurePending {
		c.inner.value = val
		c.inner.state = futureResolved
	}
}

func (c Completer[T]) Error(err error) {
	c.inner.mutex.Lock()
	defer c.inner.mutex.Unlock()
	if c.inner.state == futurePending {
		c.inner.err = err
		c.inner.state = futureRejected
	}
}

func CompleteableFuture[T any]() (Completer[T], Future[T]) {
	f := &threadsafeFuture[T]{
		state:        futurePending,
		awaitHandles: make([]struct{ signal chan<- struct{} }, 0),
	}
	return Completer[T]{inner: f}, f
}
