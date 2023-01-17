package log

import (
	"fmt"
	"strings"
)

// Fields 上下文结构，方便在结构体之间传递信息
// 模仿logrus，非线程安全
type Fields map[string]any

const (
	prefixKey = "__prefix__"
)

func (f Fields) String() string {
	str := make([]string, 1)
	for k, v := range f {
		if k == prefixKey {
			str[0] = fmt.Sprintf("[%v]", v)
		} else {
			str = append(str, fmt.Sprintf("%s=%+v", k, v))
		}
	}
	return strings.Join(str, " ")
}

func (f Fields) prepend(format string) string {
	return f.String() + "," + format
}

func (f Fields) WithPrefix(prefix string) Fields {
	return MergeFields(f, Fields{prefixKey: prefix})
}

// MergeFields 合并，至少有一个参数，结果不影响原来的数据
// 不要直接修改f，防止并发问题
func MergeFields(f Fields, fields ...Fields) Fields {
	all := make(Fields, len(f))
	for k, v := range f {
		all[k] = v
	}
	for _, field := range fields {
		for k, v := range field {
			all[k] = v
		}
	}
	return all
}

func (f Fields) WithFields(fields ...Fields) Fields {
	return MergeFields(f, fields...)
}

func (f Fields) Prefix() string {
	prefix, ok := f[prefixKey]
	if ok {
		return prefix.(string)
	}
	return ""
}

func (f Fields) Debug(format string, a ...any) {
	Debug(f.prepend(format), a...)
}

func (f Fields) Info(format string, a ...any) {
	Info(f.prepend(format), a...)
}

func (f Fields) Warn(format string, a ...any) {
	Warn(f.prepend(format), a...)
}

func (f Fields) Error(format string, a ...any) {
	Error(f.prepend(format), a...)
}

func (f Fields) Fatal(format string, a ...any) {
	Fatal(f.prepend(format), a...)
}
