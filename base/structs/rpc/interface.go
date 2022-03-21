package rpc

/**  rpc的抽象接口
  *  @author tryao
  *  @date 2022/03/21 15:05
**/

// IServer 模块间通信应该有的接口
type IServer interface {
	// Go 在module协程里异步执行命令
	Go(id any, args ...any)
	// Call0 在module协程里同步执行命令，无返回值
	Call0(id any, args ...any) error
	// Call1 在module协程里同步执行命令，有一个返回值
	Call1(id any, args ...any) (any, error)
	// CallN 在module协程里同步执行命令，有任意多个返回值
	CallN(id any, args ...any) ([]any, error)
}
