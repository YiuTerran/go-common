package nacos

import (
	"github.com/YiuTerran/go-common/base/log"
	"strings"
)

/**
  *  @author tryao
  *  @date 2022/08/16 17:20
**/

type BaseLogger struct {
}

func (b *BaseLogger) Info(args ...interface{}) {
}

func (b *BaseLogger) Warn(args ...interface{}) {
	log.Warn("", args...)
}

func (b *BaseLogger) Error(args ...interface{}) {
	log.Error("", args...)
}

func (b *BaseLogger) Debug(args ...interface{}) {
}

func (b *BaseLogger) Infof(fmt string, args ...interface{}) {
}

func (b *BaseLogger) Warnf(fmt string, args ...interface{}) {
	log.Warn(fmt, args...)
}

func (b *BaseLogger) Errorf(fmt string, args ...interface{}) {
	if strings.HasPrefix(fmt, "read cacheDir") {
		return
	}
	log.Error(fmt, args...)
}

func (b *BaseLogger) Debugf(fmt string, args ...interface{}) {
}
