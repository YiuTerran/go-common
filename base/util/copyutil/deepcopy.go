package deepcopy

import (
	"github.com/huandu/go-clone"
)

// DeepCopy 深拷贝
func DeepCopy[T any](src T) T {
	return clone.Clone(src).(T)
}

// CopyCircular 深拷贝有引用循环的数据结构
func CopyCircular[T any](src T) T {
	return clone.Slowly(src).(T)
}
