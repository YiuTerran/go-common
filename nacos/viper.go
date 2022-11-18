package nacos

import (
	"github.com/spf13/viper"
	"sync"
)

// 默认情况下viper读入配置并不是并发安全的，这里简单的包装以下

type SafeViper struct {
	lock  sync.RWMutex
	viper *viper.Viper
}

func (sv *SafeViper) Load() *viper.Viper {
	sv.lock.RLock()
	defer sv.lock.RUnlock()
	return sv.viper
}

func (sv *SafeViper) Store(vp *viper.Viper) {
	sv.lock.Lock()
	sv.viper = vp
	sv.lock.Unlock()
}
