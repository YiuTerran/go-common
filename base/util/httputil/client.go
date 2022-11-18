package httputil

import (
	"github.com/imroc/req/v3"
	"github.com/YiuTerran/go-common/base/log"
	"time"
)

var (
	// ReqLogger 结合zap实现的默认logger
	ReqLogger = &reqLogger{}
)

type reqLogger struct {
}

func (r *reqLogger) Errorf(format string, v ...any) {
	log.Error(format, v...)
}

func (r *reqLogger) Warnf(format string, v ...any) {
	log.Error(format, v...)
}

func (r *reqLogger) Debugf(format string, v ...any) {
	log.Debug(format, v...)
}

// NewClient 获取默认配置的客户端
// 如果没有特殊需求，直接用req自带的客户端即可
func NewClient() *req.Client {
	var r *req.Client
	if log.IsDebugEnabled() {
		r = req.C().EnableDumpAllAsync().SetLogger(ReqLogger)
	} else {
		r = req.C()
	}
	r.SetTimeout(3*time.Second).
		SetCommonRetryCount(3).
		SetCommonRetryBackoffInterval(100*time.Millisecond, 2*time.Second)
	return r
}

// NewRequest 使用本页配置的默认客户端发起请求
// import过httputil之后，使用req.R()也是一样的
func NewRequest() *req.Request {
	return req.R()
}

func init() {
	// 设置默认客户端
	c := NewClient()
	log.RegisterDebugSwitchCallback(func(debug bool) {
		if debug {
			c.SetLogger(ReqLogger)
			c.EnableDumpAllAsync()
		} else {
			c.SetLogger(nil)
			c.DisableDumpAll()
		}
	})
	req.SetDefaultClient(c)
}
