# golang项目基础库

类似java的common项目，所有golang项目的基础库。

基于go1.18的workspace和泛型编写，项目使用了多个go module.

由于1.18还不推荐大量使用泛型，所以仅有一些集合功能使用了泛型编写。随着时间的推移，标准库中会自己增加对应C++ STL的功能，这时就不再需要使用自建的数据结构和算法了。

## 项目结构

由于go语言不太方便做依赖倒置，也没有类似spring的框架，所以这里封装的都是一些比较简单常用的功能。

```bash
├── base  # 基础库，常用数据结构和utils，log模块
├── db # sqlx + sqlbuiler的封装
├── module # 模块封装，将所有功能统一抽象为模块，提供启动器和模块间类似rpc的相互通信机制
├── network # tcp/udp对应module的封装，提供抽象解析器，提供json的一个范例实现
├── pb # protobuf对应于network中抽象解析器的一种实现
├── redisutil # redis通信，常用功能，以及分布式锁封装
├── sip # sip通信协议，sdp等功能集成
├── ginutil # gin封装
└── ws # websocket对应于network/module的封装
```

根据go语言的规定，当多个模块写在同一个repo里时，tag需要按`<module>/<version>`的格式进行打tag，如`base/v1.0.0`，所以各个模块拥有不同的版本号。

平时开发可以直接commit然后push，在依赖库里直接使用`go get -u`升级到最新的`master`即可。最好不要在master分支直接开发。
在发布时，需要将各个库的依赖改成`latest`的稳定版本。

由于使用了workspace，所以本地编写代码时，即使`go.mod`中的依赖版本未更新，也会优先使用本地source. 如果最终push的时候忘了更新`go.mod`中依赖的版本，
会出现本地能编译但是使用者无法编译的问题。因此在每次push之前，都要检查依赖基础库的版本问题。如果是java使用snapshot的方式，由于每次都自动更新到最新，就没这个顾虑。

所以go mod的管理还是有一些问题的，需要谨慎push. 理论上这些模块使用单独的repo更好，但是由于很多模块里面的代码非常少，这样切来切去也很麻烦，所以目前采用类似java common的集中管理模式。