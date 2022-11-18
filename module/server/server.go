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

// Action 表示模块的变化
type Action int

const (
	Nothing Action = iota
	New
	Update
	Delete
)

var (
	serverCloseChn  = make(chan os.Signal, 1)
	serverReloadChn = make(chan int, 1)

	quitSig   = 1
	reloadSig = 2

	beforeCloseModuleCb func()
	afterInitModuleCb   func()
)

type GetModuleActions func() map[Action][]module.Module

// Close 手动关闭服务
func Close() {
	serverCloseChn <- os.Kill
}

// Reload 重载所有模块
func Reload() {
	serverReloadChn <- reloadSig
}

// 热加载
func hotReload(getMods GetModuleActions) {
	for {
		sig := <-serverReloadChn
		if sig == quitSig {
			break
		} else if sig == reloadSig {
			reloadByAction(getMods())
		}
	}
}

// BeforeCloseModule 服务关闭所有模块之前的hook
// 一般需要在注册中心先取消注册
func BeforeCloseModule(cb func()) {
	beforeCloseModuleCb = cb
}

// AfterInitModule 初始化所有module之后的hook
// 一般需要手动注册到注册中心
func AfterInitModule(cb func()) {
	afterInitModuleCb = cb
}

func waitClose() {
	//关闭&&重启
	signal.Notify(serverCloseChn, os.Interrupt, os.Kill)
	<-serverCloseChn
	log.Info("server closing...")
	signal.Stop(serverCloseChn)
	serverReloadChn <- quitSig
	if beforeCloseModuleCb != nil {
		beforeCloseModuleCb()
	}
	destroyAll()
	log.Info("all module closed")
}

// HotRun 开启模块热加载特性
func HotRun(getMods GetModuleActions) {
	log.Info("server starting up...")
	// mod
	reloadByAction(getMods())
	//注册热加载信号
	go hotReload(getMods)
	if afterInitModuleCb != nil {
		afterInitModuleCb()
	}
	waitClose()
}

// Run 模块以静态模式加载（关闭热加载特性）
func Run(mods ...module.Module) {
	log.Info("server starting up...")
	staticLoadModules(mods)
	if afterInitModuleCb != nil {
		afterInitModuleCb()
	}
	waitClose()
}
