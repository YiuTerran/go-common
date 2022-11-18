package numutil

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/YiuTerran/go-common/base/constraint"
	"reflect"
	"sort"
	"strconv"
)

// ConvertToInt64 guess Num format and convert to Int64
func ConvertToInt64(temp any) (int64, error) {
	switch temp.(type) {
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(temp).Int(), nil
	case uint, uint8, uint16, uint32, uint64:
		//注意这里是可能溢出的，谨慎处理
		return int64(reflect.ValueOf(temp).Uint()), nil
	case float64, float32:
		return int64(reflect.ValueOf(temp).Float()), nil
	case bool:
		if temp.(bool) {
			return 1, nil
		}
		return 0, nil
	default:
		f, e := strconv.ParseFloat(fmt.Sprint(temp), 64)
		if e != nil {
			return 0, e
		}
		return int64(f), nil
	}
}

func ConvertToBool(temp any) (bool, error) {
	switch temp.(type) {
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(temp).Int() != 0, nil
	case uint, uint8, uint16, uint32, uint64:
		return int64(reflect.ValueOf(temp).Uint()) != 0, nil
	case float64, float32:
		return int64(reflect.ValueOf(temp).Float()) != 0, nil
	case bool:
		return temp.(bool), nil
	case string:
		return temp.(string) != "", nil
	case nil:
		return false, nil
	default:
		k := reflect.TypeOf(temp).Kind()
		if k == reflect.Ptr {
			//前面已经判断了nil，所以这里指针肯定不是nil
			return true, nil
		} else if k == reflect.Map || k == reflect.Slice || k == reflect.Array {
			return reflect.ValueOf(temp).Len() > 0, nil
		}
		return false, fmt.Errorf("can't convert to int:%v", temp)
	}
}

// ConvertToFloat64 guess Num format and convert to Float64
func ConvertToFloat64(temp any) (float64, error) {
	switch temp.(type) {
	case int, int8, int16, int32, int64:
		return float64(reflect.ValueOf(temp).Int()), nil
	case uint, uint8, uint16, uint32, uint64:
		return float64(reflect.ValueOf(temp).Uint()), nil
	case float64, float32:
		return reflect.ValueOf(temp).Float(), nil
	default:
		return strconv.ParseFloat(fmt.Sprint(temp), 64)
	}
}

// Min 集合中的最小值
func Min[T constraint.Ordered](elems ...T) T {
	return lo.Min(elems)
}

// Max 集合中的最大值
func Max[T constraint.Ordered](elems ...T) T {
	return lo.Max(elems)
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
// 算法：先固定某一位的数字，再遍历其他位的可能性，递归此过程
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

// DirectProduct 任意多个集合的笛卡尔积（直积）
// 回溯法遍历所有可能性
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
