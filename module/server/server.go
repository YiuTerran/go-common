package server

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/module"
	"os"
	"os/signal"
)

/**  一般server的实现，加载所有Module
  *  @author tryao
  *  @date 2022/03/21 11:06
**/

//Action 表示模块的变化
type Action int

const (
	Nothing Action = iota
	New
	Update
	Delete
)

var (
	closeChannel    = make(chan os.Signal, 1)
	internalChannel = make(chan int, 1)

	quitSig   = 1
	reloadSig = 2
)

type GetModuleActions func() map[Action][]module.Module

// CloseServer 手动关闭服务
func CloseServer() {
	closeChannel <- os.Kill
}

// ReloadServer 重载所有模块
func ReloadServer() {
	internalChannel <- reloadSig
}

//热加载
func hotReload(getMods GetModuleActions) {
	for {
		sig := <-internalChannel
		if sig == quitSig {
			break
		} else if sig == reloadSig {
			reloadByAction(getMods())
		}
	}
}

// HotRun 开启模块热加载特性
// beforeClose是在所有模块销毁前执行的
func HotRun(getMods GetModuleActions, beforeClose func()) {
	log.Info("Server starting up...")
	// mod
	reloadByAction(getMods())
	//注册热加载信号
	go hotReload(getMods)
	//关闭&&重启
	signal.Notify(closeChannel, os.Interrupt, os.Kill)
	<-closeChannel
	internalChannel <- quitSig
	if beforeClose != nil {
		beforeClose()
	}
	signal.Stop(closeChannel)
	destroyAll()
	log.Info("Server closing down...")
}

// StaticRun 模块以静态模式加载（关闭热加载特性）
// beforeClose是在所有模块销毁前执行的
func StaticRun(mods []module.Module, beforeClose func()) {
	log.Info("Server starting up...")
	staticLoadModules(mods)
	signal.Notify(closeChannel, os.Interrupt, os.Kill)
	<-closeChannel
	if beforeClose != nil {
		beforeClose()
	}
	destroyAll()
	log.Info("Server closing down...")
}
