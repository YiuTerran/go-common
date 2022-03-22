package maputil

import "sync"

/**  lo里面有大部分需要的函数
  *  @author tryao
  *  @date 2022/03/18 14:18
**/

func SyncMapLen(m *sync.Map) int {
	size := 0
	m.Range(func(key, value any) bool {
		size++
		return true
	})
	return size
}

func Keys[K comparable, V any](m map[K]V) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

func Contains[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

func Values[K comparable, V any](m map[K]V) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}
