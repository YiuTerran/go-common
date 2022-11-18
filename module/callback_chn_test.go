package module

import (
	"fmt"
	"testing"
)

/**
  *  @author tryao
  *  @date 2022/07/27 16:31
**/

func TestNewCallbackChn(t *testing.T) {
	ch := NewCallbackChn()
	cb := func() {
		fmt.Println("callback")
	}
	ch.Go(func() {
		fmt.Println("async func")
	}, cb)
	x := <-ch.ChanCb.Out
	ch.Cb(x)
}
