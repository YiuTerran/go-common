package maputil

import "sync"

/**
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
