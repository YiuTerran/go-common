package sampleutil

import (
	"github.com/YiuTerran/go-common/base/constraint"
	"github.com/YiuTerran/go-common/base/util/arrayutil"
	"github.com/samber/lo"
	"math/rand"
)

//RandomString 随机字符串，包含大小写字母和数字
func RandomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		x := rand.Intn(3)
		switch x {
		case 0:
			bytes[i] = byte(RandInt(65, 90)) //大写字母
		case 1:
			bytes[i] = byte(RandInt(97, 122))
		case 2:
			bytes[i] = byte(rand.Intn(10))
		}
	}
	return string(bytes)
}

//RandInt 闭区间
func RandInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

func RandInt32(min, max int32) int32 {
	return min + rand.Int31n(max-min+1)
}

func RandInt64(min, max int64) int64 {
	return min + rand.Int63n(max-min+1)
}

func Shuffle[T constraint.Ordered](array []T) {
	for i := range array {
		j := rand.Intn(i + 1)
		array[i], array[j] = array[j], array[i]
	}
}

func RandChoice[T constraint.Ordered](array []T, n int) []T {
	if n <= 0 {
		return nil
	}
	return lo.Samples(array, n)
}

//WeightedChoice 根据权重随机，返回对应选项的索引，O(n)
func WeightedChoice(weightArray []int) int {
	if weightArray == nil {
		return -1
	}
	total := arrayutil.SumInts(weightArray)
	rv := rand.Int63n(total)
	for i, v := range weightArray {
		if rv < int64(v) {
			return i
		}
		rv -= int64(v)
	}
	return len(weightArray) - 1
}
