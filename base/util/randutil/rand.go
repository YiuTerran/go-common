package randutil

import (
	"github.com/samber/lo"
	"github.com/YiuTerran/go-common/base/constraint"
	"github.com/YiuTerran/go-common/base/util/arrayutil"
	"math/rand"
	"time"
)

//RandomString 随机字符串，包含大小写字母和数字

const (
	letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandString 生成长度为n的随机字符串
func RandString(n int) string {
	output := make([]byte, n)
	// We will take n bytes, one byte for each character of output.
	randomness := make([]byte, n)
	// read all random
	_, err := rand.Read(randomness)
	if err != nil {
		panic(err)
	}
	l := len(letterBytes)
	// fill output
	for pos := range output {
		// get random item
		random := randomness[pos]
		// random % 64
		randomPos := random % uint8(l)
		// put into output
		output[pos] = letterBytes[randomPos]
	}

	return string(output)
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
