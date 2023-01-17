package apm

import (
	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/YiuTerran/go-common/base/util/httputil"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

/**
  *  @author tryao
  *  @date 2022/12/09 15:44
**/

const (
	ApiNameKey = "apiName"
)

var tc propagation.TraceContext

// SetReqClientTracer 为req的http客户端添加tracer
func SetReqClientTracer(c *req.Client, tracer trace.Tracer) {
	c.WrapRoundTripFunc(func(rt req.RoundTripper) req.RoundTripFunc {
		return func(req *req.Request) (resp *req.Response, err error) {
			ctx := req.Context()
			apiName, ok := ctx.Value(ApiNameKey).(string)
			if !ok {
				apiName = req.URL.Path
			}
			_, span := tracer.Start(req.Context(), apiName)
			defer span.End()
			span.SetAttributes(
				attribute.String("http.url", req.URL.String()),
				attribute.String("http.method", req.Method),
				attribute.String("http.req.header", req.HeaderToString()),
			)
			//如果body不太大的话，记录下来
			if len(req.Body) > 0 && len(req.Body) < 10240 {
				span.SetAttributes(
					attribute.String("http.req.body", string(req.Body)),
				)
			}
			tc.Inject(ctx, propagation.HeaderCarrier(req.Headers))
			resp, err = rt.RoundTrip(req)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			if resp.Response != nil {
				span.SetAttributes(
					attribute.Int("http.status_code", resp.StatusCode),
					attribute.String("http.resp.header", resp.HeaderToString()),
					attribute.String("http.resp.body", resp.String()),
				)
			}
			return
		}
	})
}

func NewHttpClient(tracerName string) *req.Client {
	c := httputil.NewClient()
	SetReqClientTracer(c, otel.Tracer(tracerName))
	return c
}
func NewHttpRequest(tracer trace.Tracer) *req.Request {
	c := httputil.NewClient()
	SetReqClientTracer(c, tracer)
	return c.R()
}

func SetGinTracer(serviceName string, server *gin.Engine) {
	server.Use(otelgin.Middleware(serviceName))
}
