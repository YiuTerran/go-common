package wg

import (
	"github.com/samber/lo"
	"github.com/YiuTerran/go-common/base/log"
	"go.uber.org/atomic"
	"sync"
	"time"
)

/**  可以监控还剩多少job的waiter
  *  @author tryao
  *  @date 2022/04/28 17:04
**/

type WaitGroup struct {
	real    *sync.WaitGroup
	cnt     atomic.Int64
	name    string
	warnCnt atomic.Int64
	errCnt  atomic.Int64
}

func NewWaitGroup(name ...string) *WaitGroup {
	return &WaitGroup{
		name: lo.Ternary(len(name) > 0, name[0], "wg"),
		real: &sync.WaitGroup{},
		cnt:  atomic.Int64{},
	}
}

func (wg *WaitGroup) SetWarnCnt(warnCnt int64) {
	wg.warnCnt.Store(warnCnt)
}

func (wg *WaitGroup) SetErrorCnt(errCnt int64) {
	wg.errCnt.Store(errCnt)
}

func (wg *WaitGroup) Current() int64 {
	return wg.cnt.Load()
}

func (wg *WaitGroup) Wait() {
	ch := make(chan struct{}, 1)
	go func() {
		wg.real.Wait()
		ch <- struct{}{}
	}()
	for {
		select {
		case <-ch:
			return
		case <-time.After(3 * time.Second):
			log.Info("%s waiting %d task to be done...", wg.name, wg.Current())
		}
	}
}

func (wg *WaitGroup) Add(delta int) {
	wg.cnt.Add(int64(delta))
	cur := wg.cnt.Load()
	if threshold := wg.errCnt.Load(); threshold > 0 {
		if cur > threshold {
			log.Error("waitgroup %s wait %d, threshold:%d", wg.name, cur, threshold)
		}
	} else if threshold := wg.warnCnt.Load(); threshold > 0 {
		if cur > threshold {
			log.Warn("waitgroup %s wait %d, threshold:%d", wg.name, cur, threshold)
		}
	}
	wg.real.Add(delta)
}

func (wg *WaitGroup) Incr() {
	wg.Add(1)
}

func (wg *WaitGroup) Done() {
	wg.cnt.Add(-1)
	wg.real.Done()
}
