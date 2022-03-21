package ringbuffer

//copy from smallnest/ringbuffer
//正常的ring buffer是限制长度的，当队列满了就覆盖掉尾部的数据
//原来的代码buf内容固定是byte，这里改成泛型版本

import (
	"errors"
	"sync"
)

var (
	ErrTooManyDataToWrite = errors.New("too many data to write")
	ErrIsFull             = errors.New("ring buffer is full")
	ErrIsEmpty            = errors.New("ring buffer is empty")
	ErrAcquireLock        = errors.New("no lock to acquire")
)

// RingBuffer is a circular buffer that implement io.ReaderWriter interface.
type RingBuffer[T any] struct {
	buf    []T
	size   int
	r      int // next position to read
	w      int // next position to write
	isFull bool
	mu     sync.RWMutex
}

// New returns a new RingBuffer whose buffer has the given size.
func New[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		buf:  make([]T, size),
		size: size,
	}
}

// Read reads up to len(p) items into p. It returns the number of items read (0 <= n <= len(p)) and any error encountered.
// Even if Read returns n < len(p), it may use all of p as scratch space during the call.
// If some data is available but not len(p) items, Read conventionally returns what is available instead of waiting for more.
// When Read encounters an error or end-of-file condition after successfully reading n > 0 items,
// it returns the number of items read. It may return the (non-nil) error from the same call or return the error
// (and n == 0) from a subsequent call.
// Callers should always process the n > 0 items returned before considering the error err.
// Doing so correctly handles I/O errors that happen after reading some items and also both of the allowed EOF behaviors.
func (r *RingBuffer[T]) Read(p []T) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	r.mu.Lock()
	n, err = r.read(p)
	r.mu.Unlock()
	return n, err
}

// TryRead read up to len(p) items into p like Read, but it is not blocking.
// If it has not succeeded to acquire the lock, it returns 0 as n and ErrAcquireLock.
func (r *RingBuffer[T]) TryRead(p []T) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	ok := r.mu.TryLock()
	if !ok {
		return 0, ErrAcquireLock
	}

	n, err = r.read(p)
	r.mu.Unlock()
	return n, err
}

func (r *RingBuffer[T]) read(p []T) (n int, err error) {
	if r.w == r.r && !r.isFull {
		return 0, ErrIsEmpty
	}

	if r.w > r.r {
		n = r.w - r.r
		if n > len(p) {
			n = len(p)
		}
		copy(p, r.buf[r.r:r.r+n])
		r.r = (r.r + n) % r.size
		return
	}

	n = r.size - r.r + r.w
	if n > len(p) {
		n = len(p)
	}

	if r.r+n <= r.size {
		copy(p, r.buf[r.r:r.r+n])
	} else {
		c1 := r.size - r.r
		copy(p, r.buf[r.r:r.size])
		c2 := n - c1
		copy(p[c1:], r.buf[0:c2])
	}
	r.r = (r.r + n) % r.size

	r.isFull = false

	return n, err
}

// ReadItem reads and returns the next item from the input or ErrIsEmpty.
func (r *RingBuffer[T]) ReadItem() (b T, err error) {
	r.mu.Lock()
	if r.w == r.r && !r.isFull {
		r.mu.Unlock()
		err = ErrIsEmpty
		return
	}
	b = r.buf[r.r]
	r.r++
	if r.r == r.size {
		r.r = 0
	}

	r.isFull = false
	r.mu.Unlock()
	return
}

// Write writes len(p) items from p to the underlying buf.
// It returns the number of items written from p (0 <= n <= len(p)) and any error
// encountered that caused write stop early.
// Write returns a non-nil error if it returns n < len(p).
// Write must not modify the slice data, even temporarily.
func (r *RingBuffer[T]) Write(p []T) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	n, err = r.write(p)
	r.mu.Unlock()

	return n, err
}

// TryWrite writes len(p) items from p to the underlying buf like Write, but it is not blocking.
// If it has not succeeded to acquire the lock, it returns 0 as n and ErrAcquireLock.
func (r *RingBuffer[T]) TryWrite(p []T) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	ok := r.mu.TryLock()
	if !ok {
		return 0, ErrAcquireLock
	}

	n, err = r.write(p)
	r.mu.Unlock()

	return n, err
}

func (r *RingBuffer[T]) write(p []T) (n int, err error) {
	if r.isFull {
		return 0, ErrIsFull
	}

	var avail int
	if r.w >= r.r {
		avail = r.size - r.w + r.r
	} else {
		avail = r.r - r.w
	}

	if len(p) > avail {
		err = ErrTooManyDataToWrite
		p = p[:avail]
	}
	n = len(p)

	if r.w >= r.r {
		c1 := r.size - r.w
		if c1 >= n {
			copy(r.buf[r.w:], p)
			r.w += n
		} else {
			copy(r.buf[r.w:], p[:c1])
			c2 := n - c1
			copy(r.buf[0:], p[c1:])
			r.w = c2
		}
	} else {
		copy(r.buf[r.w:], p)
		r.w += n
	}

	if r.w == r.size {
		r.w = 0
	}
	if r.w == r.r {
		r.isFull = true
	}

	return n, err
}

// WriteItem writes one item into buffer, and returns ErrIsFull if buffer is full.
func (r *RingBuffer[T]) WriteItem(c T) error {
	r.mu.Lock()
	err := r.writeItem(c)
	r.mu.Unlock()
	return err
}

// TryWriteItem writes one item into buffer without blocking.
// If it has not succeeded to acquire the lock, it returns ErrAcquireLock.
func (r *RingBuffer[T]) TryWriteItem(c T) error {
	ok := r.mu.TryLock()
	if !ok {
		return ErrAcquireLock
	}
	err := r.writeItem(c)
	r.mu.Unlock()
	return err
}

func (r *RingBuffer[T]) writeItem(c T) error {
	if r.w == r.r && r.isFull {
		return ErrIsFull
	}
	r.buf[r.w] = c
	r.w++

	if r.w == r.size {
		r.w = 0
	}
	if r.w == r.r {
		r.isFull = true
	}

	return nil
}

// Length return the length of available read items.
func (r *RingBuffer) Length() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.w == r.r {
		if r.isFull {
			return r.size
		}
		return 0
	}

	if r.w > r.r {
		return r.w - r.r
	}

	return r.size - r.r + r.w
}

// Capacity returns the size of the underlying buffer.
func (r *RingBuffer) Capacity() int {
	return r.size
}

// FreeLength returns the length of available items to write.
func (r *RingBuffer) FreeLength() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.w == r.r {
		if r.isFull {
			return 0
		}
		return r.size
	}

	if r.w < r.r {
		return r.r - r.w
	}

	return r.size - r.w + r.r
}

// Items returns all available read items. It does not move the read pointer and only copy the available data.
func (r *RingBuffer[T]) Items() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.w == r.r {
		if r.isFull {
			buf := make([]T, r.size)
			copy(buf, r.buf[r.r:])
			copy(buf[r.size-r.r:], r.buf[:r.w])
			return buf
		}
		return nil
	}

	if r.w > r.r {
		buf := make([]T, r.w-r.r)
		copy(buf, r.buf[r.r:r.w])
		return buf
	}

	n := r.size - r.r + r.w
	buf := make([]T, n)

	if r.r+n < r.size {
		copy(buf, r.buf[r.r:r.r+n])
	} else {
		c1 := r.size - r.r
		copy(buf, r.buf[r.r:r.size])
		c2 := n - c1
		copy(buf[c1:], r.buf[0:c2])
	}

	return buf
}

// IsFull returns this ring-buffer is full.
func (r *RingBuffer) IsFull() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isFull
}

// IsEmpty returns this ring-buffer is empty.
func (r *RingBuffer) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return !r.isFull && r.w == r.r
}

// Reset the read pointer and writer pointer to zero.
func (r *RingBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.r = 0
	r.w = 0
	r.isFull = false
}
