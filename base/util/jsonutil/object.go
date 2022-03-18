package jsonutil

import (
	"database/sql/driver"
	"errors"
	"strings"

	"encoding/json"
	"fmt"
	"strconv"
)

//JsonObject 是一个动态的json对象
//注意：json在Unmarshal到any时，会把JsonNumber转成float64，除非使用UseNumber
//因此这里仅提供float64接口，其他数据类型外部转换
//如果json的是{type: 1, data: {}}这种格式，需要通过type解析具体的data，则推荐使用json.RawMessage来解析

type JsonObject map[string]any

var (
	TypeError  = errors.New("type convert error")
	KeyError   = errors.New("key not exist")
	IndexError = errors.New("index not exist")
)

//Value 序列化到字符串以写入db
func (j JsonObject) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

//Scan db的json字段反序列化到Json obj
func (j *JsonObject) Scan(src any) error {
	if src == nil {
		return nil
	}
	d := make(map[string]any)
	if err := json.Unmarshal(src.([]byte), &d); err != nil {
		return err
	}
	*j = d
	return nil
}

func (j JsonObject) GetInt(key string) (int, error) {
	if v, ok := j[key]; !ok || v == nil {
		return 0, fmt.Errorf("map key %s not exist", key)
	} else {
		switch v.(type) {
		case float64:
			return int(v.(float64)), nil
		case string:
			return strconv.Atoi(v.(string))
		default:
			return 0, fmt.Errorf("map key %s type error, can't convert to int", key)
		}
	}
}

func (j JsonObject) GetInt64(key string) (int64, error) {
	if v, ok := j[key]; !ok || v == nil {
		return 0, fmt.Errorf("map key %s not exist", key)
	} else {
		switch v.(type) {
		case float64:
			return int64(v.(float64)), nil
		case string:
			return strconv.ParseInt(v.(string), 0, 64)
		default:
			return 0, fmt.Errorf("map key %s type error, can't convert to int64", key)
		}
	}
}

func (j JsonObject) GetBool(key string) (bool, error) {
	if v, ok := j[key]; !ok || v == nil {
		return false, fmt.Errorf("map key %s not exist", key)
	} else {
		switch v.(type) {
		case float64:
			if int(v.(float64)) == 0 {
				return false, nil
			}
			return true, nil
		case string:
			s := v.(string)
			if s == "" || strings.ToLower(s) == "false" || strings.ToLower(s) == "no" {
				return false, nil
			}
			return true, nil
		default:
			return false, fmt.Errorf("map key %s type error, can't convert to bool", key)
		}
	}
}

func (j JsonObject) GetString(key string) (string, error) {
	if v, ok := j[key]; !ok || v == nil {
		return "", fmt.Errorf("map key %s not exist", key)
	} else {
		switch v.(type) {
		case string:
			return v.(string), nil
		case float64:
			return fmt.Sprint(v), nil
		default:
			return "", fmt.Errorf("map key %s type error, can't convert to int64", key)
		}
	}
}

func (j JsonObject) GetStringDefault(key string, def string) string {
	if v, err := j.GetString(key); err != nil {
		return def
	} else {
		return v
	}
}

func (j JsonObject) GetIntDefault(key string, def int) int {
	v, err := j.GetInt(key)
	if err == nil {
		return v

	}
	return def
}

func (j JsonObject) GetInt64Default(key string, def int64) int64 {
	v, err := j.GetInt64(key)
	if err == nil {
		return v
	}
	return def
}

func (j JsonObject) GetBoolDefault(key string, def bool) bool {
	v, err := j.GetBool(key)
	if err != nil {
		return def
	}
	return v
}

func (j JsonObject) HasKey(key string) bool {
	if _, ok := j[key]; ok {
		return true
	}
	return false
}

func (j JsonObject) HasNotNilKey(key string) bool {
	if tmp, ok := j[key]; ok {
		if tmp != nil {
			return true
		}
	}
	return false
}

func (j JsonObject) GetFloat64(key string) (float64, error) {
	var (
		tmp  any
		resp float64
		ok   bool
	)
	if tmp, ok = j[key]; ok {
		if resp, ok = tmp.(float64); ok {
			return resp, nil
		}
		return 0, TypeError
	}
	return 0, KeyError
}

func (j JsonObject) GetFloat64Default(key string, defaultValue float64) float64 {
	var (
		tmp  any
		resp float64
		ok   bool
	)
	if tmp, ok = j[key]; ok {
		if resp, ok = tmp.(float64); ok {
			return resp
		}
	}
	return defaultValue
}

func (j JsonObject) GetJsonArray(key string) (JsonArray, error) {
	var (
		tmp  any
		resp []any
		ok   bool
	)
	if tmp, ok = j[key]; ok {
		if resp, ok = tmp.([]any); ok {
			return resp, nil
		}
		return nil, TypeError
	}
	return nil, KeyError
}

func (j JsonObject) GetJsonObject(key string) (JsonObject, error) {
	var (
		tmp  any
		resp map[string]any
		ok   bool
	)
	if tmp, ok = j[key]; ok {
		if resp, ok = tmp.(map[string]any); ok {
			return resp, nil
		}
		return nil, TypeError
	}
	return nil, KeyError
}

func (j JsonObject) String() string {
	if j == nil {
		return "{}"
	}
	if x, err := json.Marshal(j); err == nil {
		return string(x)
	}
	return ""
}
