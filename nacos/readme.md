## 集成viper和nacos

golang习惯使用viper获取配置，viper本身并不支持nacos，所以需要约定一种方式从nacos中获取配置，然后再注入viper.
不过需要运行时修改配置的变量其实并不多，很多时候可以使用`viper.UnmarshalKey`将不会变更但反复使用的配置注入struct，这样可以提高性能.

只有确定会运行时变更的配置，才建议使用`viper.Get`的方式进行动态读取.

同时本服务会直接提供一个namingClient用于运行时微服务查找和负载均衡.