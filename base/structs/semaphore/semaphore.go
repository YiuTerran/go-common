package semaphore

// Forked from github.com/StefanKopieczek/gossip by @StefanKopieczek

import "sync"

// Semaphore is a Simple semaphore implementation.
// Any number of calls to Acquire() can be made; these will not block.
// If the semaphore has been acquired more times than it has been released, it is called 'blocked'.
// Otherwise, it is called 'free'.
type Semaphore interface {
	// Acquire a semaphore lock.
	Acquire()

	// Release an acquired semaphore lock.
	// This should only be called when the semaphore is blocked, otherwise behaviour is undefined
	Release()

	// Wait block execution until the semaphore is free.
	Wait()

	// Dispose Clean up the semaphore object.
	Dispose()
}

func NewSemaphore() Semaphore {
	sem := new(semaphore)
	sem.cond = sync.NewCond(&sync.Mutex{})
	go func(s *semaphore) {
		select {
		case <-s.stop:
			return
		case <-s.acquired:
			s.locks += 1
		case <-s.released:
			s.locks -= 1
			if s.locks == 0 {
				s.cond.Broadcast()
			}
		}
	}(sem)
	return sem
}

// Concrete implementation of Semaphore.
type semaphore struct {
	held     bool
	locks    int
	acquired chan struct{}
	released chan struct{}
	stop     chan struct{}
	cond     *sync.Cond
}

func (sem *semaphore) Acquire() {
	sem.acquired <- struct{}{}
}

func (sem *semaphore) Release() {
	sem.released <- struct{}{}
}

func (sem *semaphore) Wait() {
	sem.cond.L.Lock()
	for sem.locks != 0 {
		sem.cond.Wait()
	}
}

func (sem *semaphore) Dispose() {
	sem.stop <- struct{}{}
}
