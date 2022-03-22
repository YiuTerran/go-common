package jsonutil

import (
	"encoding/json"
	"github.com/YiuTerran/go-common/base/log"
)

func ToRawJson(obj any) *json.RawMessage {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	p := json.RawMessage(bytes)
	return &p
}

// MarshalString 序列化成字符串，失败返回空字符串
func MarshalString(v any) string {
	bs, err := json.Marshal(v)
	if err != nil {
		log.Error("fail to marshal as json: %+v", v)
		return ""
	}
	return string(bs)
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
