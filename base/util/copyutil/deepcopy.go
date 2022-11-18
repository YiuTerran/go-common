package copyutil

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

// CloneBytes 返回字节数组的副本
func CloneBytes(bs []byte) []byte {
	if len(bs) == 0 {
		return nil
	}
	x := make([]byte, len(bs))
	copy(x, bs)
	return x
}
