package syncmap

/**  泛型包装的sync.map
  *  @author tryao
  *  @date 2022/08/03 17:12
**/

import (
	"github.com/YiuTerran/go-common/base/util/maputil"
	"sync"
)

// Map is like a Go map[K]V but is safe for concurrent use by multiple goroutines without additional locking or coordination. Loads, stores, and deletes run in amortized constant time.
//
// The Map type is optimized for two common use cases: (1) when the entry for a given key is only ever written once but read many times, as in caches that only grow, or (2) when multiple goroutines read, write, and overwrite entries for disjoint sets of keys. In these two cases, use of a Map may significantly reduce lock contention compared to a Go map paired with a separate Mutex or RWMutex.
//
// The zero Map is empty and ready for use. A Map must not be copied after first use.
type Map[K comparable, V any] struct {
	inner sync.Map
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) {
	m.inner.Delete(key)
}

// Load returns the value stored in the map for a key, or nil if no value is present. The ok result indicates whether value was found in the map.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	val, ok := m.inner.Load(key)
	if ok {
		return val.(V), ok
	}
	var def V
	return def, ok
}

// LoadAndDelete deletes the value for a key, returning the previous value if any. The loaded result reports whether the key was present.
func (m *Map[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	val, loaded := m.inner.LoadAndDelete(key)
	if loaded {
		return val.(V), loaded
	}
	var def V
	return def, loaded
}

// LoadOrStore returns the existing value for the key if present. Otherwise, it stores and returns the given value. The loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	val, loaded := m.inner.LoadOrStore(key, value)
	return val.(V), loaded
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently (including by f), Range may reflect any
// mapping for that key from any point during the Range call. Range does not
// block other methods on the receiver; even f itself may call any method on m.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	innerFun := func(key, value any) bool {
		tKey, tVal := key.(K), value.(V)
		return f(tKey, tVal)
	}
	m.inner.Range(innerFun)
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(key K, value V) {
	m.inner.Store(key, value)
}

func (m *Map[K, V]) Size() int {
	return maputil.SyncMapLen(&m.inner)
}
