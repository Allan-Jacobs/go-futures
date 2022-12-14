package futures

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestAwaitSingleImmediate(t *testing.T) {
	future := PromiseLikeFuture(func(resolve Resolver[int], reject Rejector) {
		resolve(2)
	})

	actual, err := future.Await()
	if err != nil {
		t.Fatalf("Got an error Awaiting the future: %v", err)
	}

	expected := 2

	if actual != expected {
		t.Fatalf("expected != actual: %d != %d", expected, actual)
	}
}

func TestAwaitSingleDelayed(t *testing.T) {
	future := PromiseLikeFuture(func(resolve Resolver[int], reject Rejector) {
		time.Sleep(250 * time.Millisecond)
		resolve(2)
	})

	actual, err := future.Await()
	if err != nil {
		t.Fatalf("Got an error Awaiting the future: %v", err)
	}

	expected := 2

	if actual != expected {
		t.Fatalf("expected != actual: %d != %d", expected, actual)
	}
}

func TestAwaitMultipleImmediate(t *testing.T) {
	future := PromiseLikeFuture(func(resolve Resolver[int], reject Rejector) {
		resolve(2)
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ { // launch 20 goroutines
		wg.Add(1)
		i := i // copy for use in closure
		go func(delay time.Duration) {
			time.Sleep(delay) // random delay to simulate more real conditions
			actual, err := future.Await()
			if err != nil {
				t.Logf("Got an error Awaiting the future in goroutine %d: %v", i, err)
				t.Fail()
			}

			expected := 2

			if actual != expected {
				t.Logf("expected != actual: %d != %d", expected, actual)
				t.Fail()
			}
			wg.Done()
		}(time.Duration(rand.Intn(25) * int(time.Microsecond)))
	}
	wg.Wait()
}

func TestAwaitMultipleDelayed(t *testing.T) {
	future := PromiseLikeFuture(func(resolve Resolver[int], reject Rejector) {
		time.Sleep(250 * time.Millisecond)
		resolve(2)
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ { // launch 20 goroutines
		wg.Add(1)
		i := i // copy for use in closure
		go func(delay time.Duration) {
			time.Sleep(delay) // random delay to simulate more real conditions
			actual, err := future.Await()
			if err != nil {
				t.Logf("Got an error Awaiting the future in goroutine %d: %v", i, err)
				t.Fail()
			}

			expected := 2

			if actual != expected {
				t.Logf("expected != actual: %d != %d", expected, actual)
				t.Fail()
			}
			wg.Done()
		}(time.Duration(rand.Intn(25) * int(time.Microsecond)))
	}
	wg.Wait()
}

func TestPromiseLikeFutureResolve(t *testing.T) {
	future := PromiseLikeFuture(func(resolve Resolver[int], reject Rejector) {
		resolve(2)
	})
	res, err := future.Await()
	if err != nil {
		t.Fatalf("err != nil, got %v", err)
	}
	if res != 2 {
		t.Fatalf("result != 2, result = %d", res)
	}
}

func TestPromiseLikeFutureReject(t *testing.T) {
	expectedError := errors.New("Rejected!")
	future := PromiseLikeFuture(func(resolve Resolver[int], reject Rejector) {
		reject(expectedError)
	})
	res, err := future.Await()
	if err == nil {
		t.Fatalf("err == nil, got res: %d", res)
	}
	if !errors.Is(err, expectedError) {
		t.Fatalf("err != expectedError, got: %v", err)
	}
}

func TestGoroutineFutureResolve(t *testing.T) {
	future := GoroutineFuture(func() (int, error) {
		return 5, nil
	})
	res, err := future.Await()
	if err != nil {
		t.Fatalf("err != nil, got %v", err)
	}
	if res != 5 {
		t.Fatalf("result != 5, got: %d", res)
	}
}

func TestGoroutineFutureReject(t *testing.T) {
	expectedError := errors.New("Rejected!")
	future := GoroutineFuture(func() (int, error) {
		return 0, expectedError
	})
	res, err := future.Await()
	if err == nil {
		t.Fatalf("err == nil, got res: %d", res)
	}
	if !errors.Is(err, expectedError) {
		t.Fatalf("err != expectedError, got: %v", err)
	}
}

func TestSettledFutureResolve(t *testing.T) {
	future := ResolvedFuture(4)
	res, err := future.Await()
	if err != nil {
		t.Fatalf("err != nil, got %v", err)
	}
	if res != 4 {
		t.Fatalf("result != 4, got: %d", res)
	}
}

func TestSettledFutureReject(t *testing.T) {
	expectedError := errors.New("Rejected!")
	future := RejectedFuture[int](expectedError)
	res, err := future.Await()
	if err == nil {
		t.Fatalf("err == nil, got res: %d", res)
	}
	if !errors.Is(err, expectedError) {
		t.Fatalf("err != expectedError, got: %v", err)
	}
}
