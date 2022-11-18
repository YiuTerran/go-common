package internal

import (
	"context"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"github.com/YiuTerran/go-common/base/util/httputil"
	"github.com/YiuTerran/go-common/module/example/g"
	"net/http"
)

/**  这是一个http服务
  *  @author tryao
  *  @date 2022/07/27 10:34
**/

var (
	Module = &mod{}
)

type mod struct {
	srv *http.Server
}

func (m *mod) Name() string {
	return g.Mod2
}

func (m *mod) OnInit() {
	http.HandleFunc("/", testHandler)
	m.srv = &http.Server{
		Addr: ":9090",
	}
}

func (m *mod) Tags() []string {
	return nil
}

func (m *mod) OnDestroy() {
}

func (m *mod) Run(ctx context.Context) {
	httputil.Serv(ctx, m.srv)
}

func (m *mod) RPC() rpc.IServer {
	log.Fatal("you should not call http module directly")
	return nil
}
