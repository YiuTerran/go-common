package internal

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/module/example/g"
	"github.com/YiuTerran/go-common/module/example/m1"
	"github.com/YiuTerran/go-common/module/server"
	"net/http"
)

/**
  *  @author tryao
  *  @date 2022/07/27 10:33
**/

func testHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fn := q.Get("fn")
	p := q.Get("p")
	switch fn {
	case g.CmdAsyncExample:
		//通过名字查找模块（动态加载时）
		server.GetModuleByName(g.Mod1).RPC().Go(fn, p)
	case g.CmdCall1Example:
		//或者直接调用也行
		r, e := m1.Module().RPC().Call1(fn, p)
		log.Info("call1 resp:%v, e:%v", r, e)
	case g.CmdCallNExample:
		for _, m := range server.GetModuleByTag(g.Tag) {
			rr, e := m.RPC().CallN(fn, p)
			log.Info("calln resp:%v, e:%v", rr, e)
		}
	default:
		http.Error(w, "unknown fn", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
