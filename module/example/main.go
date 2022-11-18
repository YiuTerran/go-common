package main

import (
	"github.com/YiuTerran/go-common/module/example/m1"
	"github.com/YiuTerran/go-common/module/example/m2"
	"github.com/YiuTerran/go-common/module/server"
)

/**
  *  @author tryao
  *  @date 2022/07/27 10:31
**/

func main() {
	server.Run(
		m1.Module(),
		m2.Module,
	)
}
