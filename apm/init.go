package apm

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/util/byteutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"strings"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/12/07 16:52
**/

var (
	propagator = propagation.TraceContext{}
)

// InitProvider 初始化并返回provider
func InitProvider(param TraceParam, asGlobal bool) (*sdktrace.TracerProvider, error) {
	if param.ServiceName == "" || param.ServiceVersion == "" {
		return nil, fmt.Errorf("invalid service param to init tracer")
	}
	//如果是k8s环境，应当使用pod id
	//非k8s环境，一般用ip地址+端口
	if param.ServiceInstance == "" {
		param.ServiceInstance = byteutil.SimpleUUID4()
	}
	//默认是生产环境
	if param.Environment == "" {
		param.Environment = "prod"
	}
	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(param.ServiceName),
			semconv.ServiceVersionKey.String(param.ServiceVersion),
			semconv.ServiceInstanceIDKey.String(param.ServiceInstance),
			semconv.DeploymentEnvironmentKey.String(param.Environment),
		),
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		return nil, fmt.Errorf("fail to create resource:%w", err)
	}
	var exporter sdktrace.SpanExporter
	if param.Endpoint == "" {
		//兜底策略，控制台输出
		exporter, err = stdouttrace.New(
			stdouttrace.WithWriter(os.Stdout),
			stdouttrace.WithPrettyPrint(),
		)
	} else if param.Protocol == ProtocolGRPC {
		//连接collector超时时间
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		opts := []grpc.DialOption{grpc.WithBlock()}
		if !param.EnableTLS {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			//tls证书
			var creds credentials.TransportCredentials
			if param.CertFile != "" {
				creds, err = credentials.NewClientTLSFromFile(param.CertFile, "")
				if err != nil {
					return nil, fmt.Errorf("init grpc conn to apm server err:%s", err)
				}
			} else {
				return nil, fmt.Errorf("you should specific CertFile when enable tls with grpc")
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}
		conn, err := grpc.DialContext(ctx, param.Endpoint, opts...)
		if err != nil {
			log.Fatal("fail to init grpc conn to apm server:%s", err)
		}
		exporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("fail to create trace exporter:%w", err)
		}
	} else {
		var tlsConf otlptracehttp.Option
		if param.EnableTLS {
			tlsConf = otlptracehttp.WithTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		} else {
			tlsConf = otlptracehttp.WithInsecure()
		}
		exporter, err = otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(param.Endpoint),
			tlsConf,
		)
		if err != nil {
			return nil, fmt.Errorf("fail to create http exporter:%w", err)
		}
	}
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(
			sdktrace.ParentBased(sdktrace.TraceIDRatioBased(param.SampleRate)),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	if asGlobal {
		otel.SetTracerProvider(traceProvider)
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Warn("apm error:%s", err)
	}))
	return traceProvider, nil
}

// NewSpanContextFromParent 通过已有的traceId和父spanId来创建一个spanContext
func NewSpanContextFromParent(traceId, spanId string) (spanCxt trace.SpanContext, err error) {
	var tid trace.TraceID
	var config trace.SpanContextConfig
	tid, err = trace.TraceIDFromHex(traceId)
	if err != nil {
		return spanCxt, err
	}
	config.TraceID = tid
	if len(spanId) > 0 {
		var sid trace.SpanID
		sid, err = trace.SpanIDFromHex(spanId)
		if err != nil {
			return spanCxt, err
		}
		config.SpanID = sid
	}
	spanCxt = trace.NewSpanContext(config)
	return spanCxt, nil
}

// ExtractW3CBinaryTraceParent 从w3c trace-context-binary 格式中解析trace parent
// [link](https://github.com/w3c/trace-context-binary/blob/571cafae56360d99c1f233e7df7d0009b44201fe/spec/20-binary-format.md)
func ExtractW3CBinaryTraceParent(bs []byte) (spanCxt trace.SpanContext, err error) {
	if len(bs) < 29 {
		err = fmt.Errorf("trace parent length error")
		return
	}
	version := bs[0]
	switch version {
	case 0:
		if bs[1] != 0 || bs[18] != 1 || bs[27] != 2 {
			return spanCxt, fmt.Errorf("format error")
		}
		spanCxt = spanCxt.WithTraceID(*(*[16]byte)(bs[2:18])).
			WithSpanID(*(*[8]byte)(bs[19:27])).
			WithTraceFlags(trace.TraceFlags(bs[28]))
		return spanCxt, err
	default:
		return spanCxt, fmt.Errorf("unknown trace version")
	}
}

// ExtractW3CBinaryTraceState 从w3c trace-context-binary 格式中解析trace state
// 对应的字符串格式是k1=v1,k2=k2
func ExtractW3CBinaryTraceState(bs []byte) (state trace.TraceState, err error) {
	if len(bs) <= 2 {
		return state, nil
	}
	idx := 0
	kvs := make([]string, 0)
	for {
		if idx >= len(bs) {
			break
		}
		if bs[idx] != 0 {
			return state, fmt.Errorf("format error")
		}
		keyLen := int(bs[idx+1])
		if keyLen == 0 {
			break
		}
		key := string(bs[idx+2 : idx+2+keyLen])
		valueLen := int(bs[idx+2+keyLen])
		value := string(bs[idx+keyLen+3 : idx+keyLen+3+valueLen])
		kvs = append(kvs, fmt.Sprintf("%s=%s", key, value))
		idx = idx + keyLen + 3 + valueLen
	}
	return trace.ParseTraceState(strings.Join(kvs, ","))
}

func ExtractW3CTextTrace(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return propagator.Extract(ctx, carrier)
}

func InjectW3CTextTrace(ctx context.Context, carrier propagation.TextMapCarrier) {
	propagator.Inject(ctx, carrier)
}

// ToW3CBinary 序列化成w3c二进制
func ToW3CBinary(sxt trace.SpanContext) (traceParent, traceState []byte) {
	traceParent = make([]byte, 29)
	traceParent[18] = 1
	traceParent[27] = 2
	traceId := [16]byte(sxt.TraceID())
	copy(traceParent[2:18], traceId[:])
	if sxt.SpanID().IsValid() {
		spanId := [8]byte(sxt.SpanID())
		copy(traceParent[19:27], spanId[:])
	}
	traceParent[28] = byte(sxt.TraceFlags())
	if sxt.TraceState().Len() > 0 {
		s := sxt.TraceState().String()
		pairs := strings.Split(s, ",")
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				continue
			}
			traceState = append(traceState, 0, byte(len(kv[0])))
			traceState = append(traceState, []byte(kv[0])...)
			traceState = append(traceState, byte(len(kv[1])))
			traceState = append(traceState, []byte(kv[1])...)
		}
	} else {
		traceState = []byte{0, 0}
	}
	return
}

// ExtractPureContext 移除掉context的cancel和timeout等配置，仅保留trace信息
func ExtractPureContext(ctx context.Context) context.Context {
	spanCtx := trace.SpanContextFromContext(ctx)
	return trace.ContextWithSpanContext(context.Background(), spanCtx)
}
