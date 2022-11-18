package httputil

import (
	"context"
	"errors"
	"github.com/YiuTerran/go-common/base/log"
	"net/http"
)

/**
  *  @author tryao
  *  @date 2022/07/27 14:59
**/

// Serv 启动服务，并在ctx结束时优雅关闭服务
// 这个封装的目的是保证该函数退出时，服务能够正常优雅关闭
func Serv(ctx context.Context, server *http.Server) {
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.Background()); err != nil {
			log.Error("fail to shutdown http server:%v", err)
		}
		done <- struct{}{}
	}()
	//当shutdown调用时，这里会立刻返回
	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Error("fail to start server:%v", err)
	}
	//等待shutdown执行完毕
	<-done
}
