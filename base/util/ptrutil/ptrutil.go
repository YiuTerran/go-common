package ptrutil

/**
  *  @author tryao
  *  @date 2022/03/25 08:58
**/

// Equal 判断指针指向的target是否相等
//假设确实能明确比较时使用此函数
func Equal[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// MustNotEqual 一定不相等
//某一个为nil，但并不是两个都为nil时返回true
func MustNotEqual(a, b any) bool {
	if a == nil || b == nil {
		if a != nil || b != nil {
			return true
		}
	}
	return false
}

// Coalesce 返回第一个非nil的值
func Coalesce(args ...any) any {
	for _, arg := range args {
		if arg != nil {
			return arg
		}
	}
	return nil
}
