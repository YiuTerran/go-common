package udp

import (
	"errors"
)

//udp是无连接的，直接将字节流读入读出即可
//操作系统将客户端一次发过来的包一次性放入缓冲区，因此没有所谓的粘包问题；go使用了linux的UDP设计实现，bind本质上只是绑定了一个地址
//但是单次包最大长度是有限制的(理论最大值：65507，实际最大值1472)，如果超出这个限制，就要拆包；但是由于UDP不可靠，这种情况下就要自己实现TCP的很多功能
//另外，由于MTU的限制，UDP的长度最好在512字节之内（参考http://dwz.win/vN5)

const (
	MaxPacketSize = 65535
	// SafePackageSize udp字节包安全长度
	SafePackageSize = 1500
)

var (
	InitError = errors.New("fail to init")
)
