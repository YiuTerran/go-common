package ginutil

/**  gin日志的一些简单封装
  *  @author tryao
  *  @date 2022/03/22 14:50
**/

import (
	"bytes"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/samber/lo"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config is config setting for Ginzap
type Config struct {
	SkipPaths []string
}

//从http请求中复制出body，注意避免文件上传的场景
func peakBody(c *gin.Context) string {
	if (c.Request.Method == "PUT" || c.Request.Method == "POST") &&
		!lo.Contains(c.Request.Header["Content-Type"], "multipart/form-data") {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil && err != io.EOF {
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
			return string(body)
		}
	}
	return ""
}

func AccessLogHandler(jsonFormat bool, withBody bool, skipPath ...string) gin.HandlerFunc {
	sp := make(map[string]bool, len(skipPath))
	for _, path := range skipPath {
		sp[path] = true
	}

	return func(c *gin.Context) {
		start := time.Now()
		// some evil middlewares modify this values
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		if _, ok := sp[path]; !ok {
			end := time.Now()
			latency := end.Sub(start)

			if len(c.Errors) > 0 {
				// Append error field if this is an erroneous request.
				for _, e := range c.Errors.Errors() {
					if jsonFormat {
						log.JsonError(e)
					} else {
						log.Error(e)
					}
				}
			} else {
				if jsonFormat {
					fields := []zapcore.Field{
						zap.String("method", c.Request.Method),
						zap.String("path", path),
						zap.String("query", query),
						zap.Int("status", c.Writer.Status()),
						zap.String("ip", c.ClientIP()),
						zap.String("user-agent", c.Request.UserAgent()),
						zap.Duration("latency", latency),
					}
					//打印body，一般不需要
					if withBody {
						body := peakBody(c)
						if body != "" {
							fields = append(fields, zap.String("body", body))
						}
					}
					log.JsonDebug(path, fields...)
				} else {
					txt := fmt.Sprintf("%s %s Q:%s ST:%d IP:%s UA:%s LAT:%s", c.Request.Method, path, query, c.Writer.Status(),
						c.ClientIP(), c.Request.UserAgent(), latency)
					if withBody {
						body := peakBody(c)
						if body != "" {
							txt += " BODY:" + body
						}
					}
					log.Debug(txt)
				}
			}
		}
	}
}

func RecoveryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					log.Error("panic when request %s, error:%s, req:%s", c.Request.URL.Path, err, httpRequest)
					// If the connection is dead, we can't write a status to it.
					_ = c.Error(err.(error)) // nolint: err check
					c.Abort()
					return
				}

				log.PanicStack("gin panic, request: "+string(httpRequest), err)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
