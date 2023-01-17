package apm

/**
  *  @author tryao
  *  @date 2022/12/07 16:52
**/
const (
	ProtocolHTTP = "http"
	ProtocolGRPC = "grpc"
)

type TraceParam struct {
	ServiceName     string  // 服务名称
	ServiceVersion  string  // 版本号，避免服务版本不一致问题
	ServiceInstance string  // 示例标识，pod id或者ip:port之类的
	Environment     string  // dev, test, prod之类
	Endpoint        string  // host:port
	Authorization   string  // basic auth或者api key
	Protocol        string  // http或者grpc
	EnableTLS       bool    //是否使用ssl
	CertFile        string  //证书路径，使用tls时才需要配置
	SampleRate      float64 //默认为1
}
