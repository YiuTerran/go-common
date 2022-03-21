package byteutil

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func byteToString(bs []byte, delim string, formatter string) string {
	var sb strings.Builder
	for i, b := range bs {
		if i != 0 && delim != "" {
			sb.WriteString(delim)
		}
		sb.WriteString(fmt.Sprintf(formatter, b))
	}
	return sb.String()
}

// BytesToIntString 将字节数组转换成10进制组成的字符串（如IP地址转换）
func BytesToIntString(bs []byte, delim string) string {
	return byteToString(bs, delim, "%d")
}

// BytesToHexString 将字节数组转成16进制组成的字符串（如MAC地址转换），强制使用
func BytesToHexString(bs []byte, delim string) string {
	return byteToString(bs, delim, "%02X")
}

func stringToByte(s string, delim string, base int) []byte {
	ss := strings.Split(s, delim)
	result := make([]byte, 0)
	for _, s := range ss {
		i, _ := strconv.ParseInt(s, base, 9) //uint8是9位
		result = append(result, byte(i))
	}
	return result
}

// IntStringToBytes 将整数组成的字符串转成字节数组
func IntStringToBytes(s string, delim string) []byte {
	return stringToByte(s, delim, 10)
}

// HexStringToBytes 将16进制数组成的字符串转成字节数组
func HexStringToBytes(s string, delim string) []byte {
	return stringToByte(s, delim, 16)
}

//RemoveUUIDDash 移除uuid中的dash
func RemoveUUIDDash(uid string) string {
	return strings.Join(strings.Split(uid, "-"), "")
}

//Md5HexString 计算md5，结果是16进制hex
func Md5HexString(bs []byte) string {
	m := md5.New()
	m.Write(bs)
	return hex.EncodeToString(m.Sum(nil))
}

//UUID4 uuid4转成string
func UUID4() string {
	u, _ := uuid.NewRandom()
	return u.String()
}

//SimpleUUID4 uuid4，不带dash
func SimpleUUID4() string {
	return RemoveUUIDDash(UUID4())
}

func Md5(param ...any) string {
	m := md5.New()
	var ss strings.Builder
	for _, p := range param {
		ss.WriteString(fmt.Sprint(p))
	}
	m.Write([]byte(ss.String()))
	return hex.EncodeToString(m.Sum(nil))
}

//AdjustByteSlice 调整slice的长度到size，如果不足则右侧补0，超出则截断
func AdjustByteSlice(src []byte, size int) []byte {
	if len(src) == size {
		return src
	}
	if len(src) < size {
		arr := make([]byte, size)
		copy(arr, src)
		return arr
	}
	return src[:size]
}

//MergeBytes 将二维byte数组压平
func MergeBytes(bs [][]byte) []byte {
	if len(bs) == 0 {
		return []byte{}
	}
	if len(bs) == 1 {
		return bs[0]
	}
	l := 0
	for i := 0; i < len(bs); i++ {
		l += len(bs[i])
	}
	buffer := make([]byte, l)
	l = 0
	for i := 0; i < len(bs); i++ {
		copy(buffer[l:], bs[i])
		l += len(bs[i])
	}
	return buffer
}
