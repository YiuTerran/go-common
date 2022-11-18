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

// Contains 判断是否有元素
func Contains[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

// Merge 合并多个map，如果有相同的k，右边的会覆盖左边的
func Merge[K comparable, V any](ms ...map[K]V) map[K]V {
	resp := make(map[K]V, len(ms))
	for _, m := range ms {
		for k, v := range m {
			resp[k] = v
		}
	}
	return resp
}

func Pop[K comparable, V any](m map[K]V, k K) (V, bool) {
	v, ok := m[k]
	if !ok {
		return v, false
	}
	delete(m, k)
	return v, true
}
