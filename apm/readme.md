# 分布式链路追踪基础库

这里为常用的库增加了分布式链路追踪的注入，包括：

1. 初始化全局trace provider
2. gin对应的http服务，以及req库对应的http客户端，这些都比较常用

redis相关包如下（避免依赖污染）：
```go
package example

// HookRedis from github.com/go-redis/redis/extra/redisotel/v8
func HookRedis(conn redis.UniversalClient) {
	conn.AddHook(redisotel.NewTracingHook())
}
```

MQ一般通过header注入，可以参考gb28181里面的`ExtractKafkaTraceCxt`和`InjectKafkaTraceCxt`