package module

import (
	"context"
	"github.com/YiuTerran/go-common/base/structs/rpc"
)

//Module 是抽象的运行模块
type Module interface {
	// Name 是模块的名字，或者说标识；类似spring的bean标示
	//module的Name不能重复
	Name() string
	// OnInit 初始化的回调
	OnInit()
	// Tags 对模块的标记，方便查找
	//Tag可以重复
	Tags() []string
	// OnDestroy 模块销毁前的回调
	OnDestroy()
	// Run 主循环，请实现一个阻塞函数，一般是一个死循环
	Run(ctx context.Context)
	// RPC 用于支持模块间通信
	// 可以使用GoroutineMixIn作为默认实现
	RPC() rpc.IServer
}
