package internal

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/module"
	"github.com/YiuTerran/go-common/module/example/g"
	"sync"
)

/**  示例mod1
  *  @author tryao
  *  @date 2022/07/27 10:34
**/

var (
	once sync.Once
	m    *mod
)

func Instance() module.Module {
	once.Do(func() {
		m = &mod{}
	})
	return m
}

type mod struct {
	// 模块主协程
	*module.GoroutineMixIn
}

func (m *mod) Name() string {
	return g.Mod1
}

func (m *mod) OnInit() {
	log.Info("module 1 init")
	m.GoroutineMixIn = module.NewGoroutineMixIn()
	m.Register(g.CmdAsyncExample, async)
	m.Register(g.CmdCall1Example, call1)
	m.Register(g.CmdCallNExample, callN)
}

func (m *mod) Tags() []string {
	return []string{g.Tag}
}
