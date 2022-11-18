package timeutil

import (
	"github.com/YiuTerran/go-common/base/log"
	"testing"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/06/28 09:24
**/

func TestCycleCount(t *testing.T) {
	<-CycleCount(func() {
		log.Info("hello")
	}, 15*time.Second, 3)
}
