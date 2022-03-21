package module

// IRpcServer 模块间通信应该有的接口
type IRpcServer interface {
	// Go 在module协程里异步执行命令
	Go(id any, args ...any)
	// Call0 在module协程里同步执行命令，无返回值
	Call0(id any, args ...any) error
	// Call1 在module协程里同步执行命令，有一个返回值
	Call1(id any, args ...any) (any, error)
	// CallN 在module协程里同步执行命令，有任意多个返回值
	CallN(id any, args ...any) ([]any, error)
}

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
	// Run 主循环，closeSig是退出信号
	// 可以使用GoroutineMixIn的Run作为默认循环
	Run(closeSig chan struct{})
	// RpcServer 用于支持模块间通信
	// 可以使用GoroutineMixIn作为默认实现
	RpcServer() IRpcServer
}
