package httputil

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/imroc/req/v3"
	"time"
)

// Client 获取默认配置的客户端
func Client() *req.Client {
	var r *req.Client
	if log.IsDebugEnabled() {
		r = req.C().EnableDebugLog()
	} else {
		r = req.C()
	}
	r.SetTimeout(5 * time.Second).
		SetCommonRetryCount(3).
		SetCommonRetryFixedInterval(2 * time.Second)
	return r
}

func Request() *req.Request {
	return Client().R()
}

func init() {
	req.SetDefaultClient(Client())
}
