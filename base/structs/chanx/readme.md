## 无限长的channel

fork from `smallnest/chanx`

一般服务较难确认缓冲区最大长度，根据负载进行弹性伸缩是比较合理的设计，因此common里面大部分服务都是用这个无限长的队列而不是一般的`chan`。