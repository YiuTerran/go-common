package module

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/chanx"
	"time"
)

// Dispatcher one per goroutine (goroutine not safe)
type Dispatcher struct {
	ChanTimer *chanx.UnboundedChan[*Timer]
}

func NewDispatcher() *Dispatcher {
	dp := new(Dispatcher)
	dp.ChanTimer = chanx.NewUnboundedChan[*Timer](initBufferSize)
	return dp
}

// Timer 定时器
type Timer struct {
	t  *time.Timer
	cb func()
}

func (t *Timer) Stop() {
	t.t.Stop()
	t.cb = nil
}

func (t *Timer) Cb() {
	defer func() {
		t.cb = nil
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()

	if t.cb != nil {
		t.cb()
	}
}

func (dp *Dispatcher) AfterFunc(d time.Duration, cb func()) *Timer {
	t := new(Timer)
	t.cb = cb
	t.t = time.AfterFunc(d, func() {
		dp.ChanTimer.In <- t
	})
	return t
}

type Cron struct {
	t *Timer
}

func (c *Cron) Stop() {
	if c.t != nil {
		c.t.Stop()
	}
}

func (dp *Dispatcher) CronFunc(cronExpr *CronExpr, _cb func()) *Cron {
	c := new(Cron)

	now := time.Now()
	nextTime := cronExpr.Next(now)
	if nextTime.IsZero() {
		return c
	}

	// callback
	var cb func()
	cb = func() {
		defer _cb()

		now := time.Now()
		nextTime := cronExpr.Next(now)
		if nextTime.IsZero() {
			return
		}
		c.t = dp.AfterFunc(nextTime.Sub(now), cb)
	}

	c.t = dp.AfterFunc(nextTime.Sub(now), cb)
	return c
}
