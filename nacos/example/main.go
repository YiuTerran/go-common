package main

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/nacos"
	"os"
	"os/signal"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/04/08 15:42
**/

func main() {
	exitCh := make(chan os.Signal, 1)
	nacos.Init()
	vp1, ch1 := nacos.GetDefaultViper()
	vp2, ch2 := nacos.GetViper("DEFAULT_GROUP", "hello-local.yaml")
	if vp1 == nil || vp2 == nil {
		return
	}
	fn := func(name string, vp *nacos.SafeViper) {
		for {
			log.Debug("%s :%s", name, vp.Load().GetString("hello.world"))
			time.Sleep(3 * time.Second)
		}
	}
	go fn("vp1", vp1)
	go fn("vp2", vp2)
	signal.Notify(exitCh, os.Interrupt, os.Kill)
	<-exitCh
	close(ch1)
	close(ch2)
}
