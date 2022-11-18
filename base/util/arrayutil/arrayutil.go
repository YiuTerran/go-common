package arrayutil

import (
	"github.com/YiuTerran/go-common/base/constraint"
)

/**
  *  @author tryao
  *  @date 2022/03/18 12:29
**/

//Sum 任意数值类型数组求和
func Sum[T constraint.Number](array []T) float64 {
	var result float64
	for _, t := range array {
		result += float64(t)
	}
	return result
}

//SumInts 整数数组求和
func SumInts[T constraint.Integer](array []T) int64 {
	var result int64
	for _, t := range array {
		result += int64(t)
	}
	return result
}

// Prepend 与append相反，将a放在最前面
//注意a放置时仍然是按顺序放置的
func Prepend[T any](array []T, a ...T) []T {
	return append(a, array...)
}
