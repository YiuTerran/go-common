package internal

import (
	"github.com/YiuTerran/go-common/base/log"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/07/27 10:34
**/

func async(a []any) {
	log.Info("async hello %v", a)
}

func call1(a []any) any {
	ch := make(chan string, 1)
	m.Go(func() {
		log.Info("receive call1@%v", time.Now().String())
		time.Sleep(3 * time.Second)
		ch <- time.Now().String()
	}, nil)
	return <-ch
}

func callN(a []any) []any {
	log.Info("receive callN@%s", time.Now().String())
	if len(a) > 0 {
		word := a[0].(string)
		return []any{"hello", word}
	}
	return nil
}
