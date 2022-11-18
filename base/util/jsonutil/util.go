package jsonutil

import (
	"encoding/json"
	"github.com/YiuTerran/go-common/base/log"
)

func Unmarshal[T any](data []byte) *T {
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		log.Error("fail to unmarshal %s", string(data))
		return nil
	}
	return &t
}

// MarshalString 序列化成字符串，失败返回空字符串
func MarshalString(v any) string {
	return string(Marshal(v))
}

func Marshal(v any) []byte {
	bs, err := json.Marshal(v)
	if err != nil {
		log.Error("fail to marshal as json: %+v", v)
		return []byte("")
	}
	return bs
}

// MarshalBeautify 格式化的序列化，方便debug查看
func MarshalBeautify(v any) string {
	bs, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		log.Error("fail to marshal as json: %+v", v)
		return ""
	}
	return string(bs)
}
