package numutil

import (
	"fmt"
	"reflect"
	"sort"
	"video.hermes.odin/go-common/base/constraint"
)

//ConvertToInt guess Num format and convert to Int
func ConvertToInt(temp any) (int, error) {
	switch t := temp.(type) {
	case int:
		return t, nil
	case float64, float32:
		return int(reflect.ValueOf(t).Float()), nil
	case int64, int32:
		return int(reflect.ValueOf(t).Int()), nil
	default:
		return 0, fmt.Errorf("can't convert to int:%v", temp)
	}
}

var floatType = reflect.TypeOf(float64(0))

//ConvertToFloat64 guess Num format and convert to Float64
func ConvertToFloat64(unk any) (float64, error) {
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)
	if !v.Type().ConvertibleTo(floatType) {
		return 0, fmt.Errorf("cannot convert %v to float64", v.Type())
	}
	fv := v.Convert(floatType)
	return fv.Float(), nil
}

func Min[T constraint.Ordered](x, y T) T {
	if x < y {
		return x
	}
	return y
}

func Max[T constraint.Ordered](x, y T) T {
	if x > y {
		return x
	}
	return y
}

func gcd(a, b int) int {
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}

// div : divide by gcd
func div(a, b int) (a0, b0 int) {
	gcd := gcd(a, b)
	a /= gcd
	b /= gcd
	return a, b
}

// C 计算组合结果
func C(n, k int) int {
	i := k + 1
	r := n - k
	if r > k {
		i = r + 1
		r = k
	}
	f1, f2 := 1, 1
	j := 1
	for ; i <= n; i++ {
		f1 *= i
		for ; j <= r; j++ {
			f2 *= j
			if f2 > f1 {
				j++
				break
			}
			if gcd := gcd(f1, f2); gcd > 1 {
				f1, f2 = div(f1, f2)
			}
		}
	}
	return f1 / f2
}

// Permutations 全排列
func Permutations[T any](arr []T) [][]T {
	var helper func([]T, int)
	var res [][]T

	helper = func(arr []T, n int) {
		if n == 1 {
			tmp := make([]T, len(arr))
			copy(tmp, arr)
			res = append(res, tmp)
		} else {
			for i := 0; i < n; i++ {
				helper(arr, n-1)
				if n%2 == 1 {
					tmp := arr[i]
					arr[i] = arr[n-1]
					arr[n-1] = tmp
				} else {
					tmp := arr[0]
					arr[0] = arr[n-1]
					arr[n-1] = tmp
				}
			}
		}
	}
	helper(arr, len(arr))
	return res
}

// Combinations 从数组中选出m个任意组合
//算法：先固定某一位的数字，再遍历其他位的可能性，递归此过程
func Combinations[T constraint.Ordered](arr []T, m int) [][]T {
	if arr == nil || m > len(arr) || m <= 0 {
		return nil
	}
	result := make([][]T, 0, C(len(arr), m))
	data := make([]T, m)
	var helper func(int, int, int)

	helper = func(start int, end int, index int) {
		if index == m {
			d := make([]T, m)
			copy(d, data)
			result = append(result, d)
			return
		}
		for i := start; i < end && end-i+1 >= m-index; i++ {
			data[index] = arr[i]
			helper(i+1, end, index+1)
			//去重
			for i+1 < end && arr[i] == arr[i+1] {
				i++
			}
		}
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i] < arr[j]
	})
	helper(0, len(arr), 0)
	return result
}

//DirectProduct 任意多个集合的笛卡尔积（直积）
//回溯法遍历所有可能性
func DirectProduct[T any](items ...[]T) [][]T {
	if len(items) == 0 {
		return nil
	}
	size := 1
	for _, item := range items {
		size *= len(item)
	}
	result := make([][]T, 0, size)
	data := make([]T, len(items))
	var backtrack func(int)
	backtrack = func(index int) {
		if len(items) == index {
			d := make([]T, len(items))
			copy(d, data)
			result = append(result, d)
			return
		}
		for i := 0; i < len(items[index]); i++ {
			data[index] = items[index][i]
			backtrack(index + 1)
		}
	}
	backtrack(0)
	return result
}

// Range 生成一个从start到end-1的数组
func Range[T constraint.Integer](start, end T) []T {
	if start >= end {
		return nil
	}
	result := make([]T, 0, end-start)
	for start < end {
		result = append(result, start)
		start++
	}
	return result
}
