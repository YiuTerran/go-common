# 网络通信抽象

一般情况下，网络通信可以抽象为一条连接上的一个会话。

所以interface里面定义了`Session`表示会话，`Conn`表示连接，`MsgProcessor`表示消息序列化和反序列化，以及消息路由工具。`agent`包里面还有socket代理的一些抽象接口（分开是为了避免循环依赖）。并给出了`SessionAgentImpl`这个实现同时满足`Conn`和`Agent`的实现。

这套逻辑主要适配于自定义协议，即裸TCP/UDP或者Websocket的写法。

如果是SIP/HTTP/Socket.io等应用层高级协议，一般有自己的抽象方式，不建议使用该库进行处理，因为再次封装意义不大。

## Conn & Agent

包里内置了tcp/udp两种实现。使用`gate`包里面的`TcpGate`和`UdpGate`就能方便的创建一个实现了消息分发、消息解析、模块间通信的网关服务。

`go-common`里面还有一个`ws`包，这是websocket的实现。

自定义协议一般使用protobuf或者json，`processor`包里给出了json方式的实现。另外有一个pb包，实现了protobuf对应tcp的解析器。

由于udp封包限长，正常是不建议使用protobuf的，建议使用json分包传输。而且udp需要加上应用层确认重发机制，这里udp的封装并未考虑这些应用层的需求。

## MsgProcessor

报文的解析器的抽象，需要支持数据序列化和反序列化，以及路由分发。

包里内置了json类型消息的处理器。

`go-common`下的pb包，则是protobuf版本的封装。