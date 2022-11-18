package ginutil

import (
	"github.com/gin-gonic/gin"
)

/**
  *  @author tryao
  *  @date 2022/07/25 15:58
**/

// InitRouter 创建一个激活常用配置的router
// 这里没有注入prometheus，因为不一定会使用
func InitRouter(allowHeaders ...string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	logHandler := AccessLogHandler(false, true, "/metrics")
	router.Use(logHandler)
	router.Use(RecoveryHandler())
	router.Use(CorsHandler(allowHeaders...))
	EnablePProf(router)
	EnableLogSwitch(router)
	return router
}
