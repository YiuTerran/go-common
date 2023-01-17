package apm

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

/**  由于OpenTelemetry本来就有grpc相关包的依赖，所以这里把相关参数也加在这里
  *  @author tryao
  *  @date 2022/12/29 09:28
**/

// GrpcClientOpts 是grpc客户端dial时的注入参数
func GrpcClientOpts() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	}
}

// GrpcServerOpts 是GRPC服务端创建时注入的参数
// 在服务端函数里可以根据需求创建span
func GrpcServerOpts() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	}
}
