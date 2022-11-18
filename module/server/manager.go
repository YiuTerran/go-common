package server

import (
	"context"
	"fmt"
	"github.com/samber/lo"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/set"
	"github.com/YiuTerran/go-common/base/structs/wg"
	"github.com/YiuTerran/go-common/module"
	"sync"
)

/**
  *  @author tryao
  *  @date 2022/03/21 11:22
**/

type mod struct {
	mi       module.Module
	cancelFn context.CancelFunc
	wg       *wg.WaitGroup
}

var (
	ordered    = make([]*mod, 0, 1)
	mods       = make(map[string]*mod)
	tags       = make(map[string]*set.Set[string])
	lock       sync.RWMutex
	staticMode bool //静态模式
)

// GetModuleByName 通过名称查找mod，类似spring查找bean
func GetModuleByName(name string) module.Module {
	return GetModuleByNameFunc(name)()
}

// ForEachModule 有tag的每个模块，异步执行函数
func ForEachModule(tag []string, id any, args ...any) {
	var ts []module.Module
	if len(tag) > 0 {
		ts = GetModuleByTag(tag...)
	} else {
		for _, m := range mods {
			ts = append(ts, m.mi)
		}
	}
	for _, t := range ts {
		t.RPC().Go(id, args...)
	}
}

// GetModuleByNameFunc 热加载时module是会变的，所以返回一个函数
// 高阶函数
func GetModuleByNameFunc(name string) func() module.Module {
	return func() module.Module {
		lock.RLock()
		defer lock.RUnlock()
		m, ok := mods[name]
		if !ok {
			return nil
		}
		return m.mi
	}
}

// GetModuleByTag 通过Tag查找模块
// 可以传入多个tag，取交集
func GetModuleByTag(tag ...string) []module.Module {
	return GetModuleByTagFunc(tag...)()
}

// GetModuleByTagFunc 获取一个函数，执行可以动态获取tag对应的module
// 适用于Module会动态热加载的场景
func GetModuleByTagFunc(tag ...string) func() []module.Module {
	return func() []module.Module {
		lock.RLock()
		defer lock.RUnlock()
		s := set.NewSet[string]()
		for i, t := range tag {
			m, ok := tags[t]
			if !ok {
				return nil
			}
			if i == 0 {
				s.AddItem(m.ToArray()...)
			} else {
				//取交集
				s = lo.ToPtr(s.Intersect(m))
			}
			if s.Size() == 0 {
				return nil
			}
		}
		resp := make([]module.Module, 0, s.Size())
		s.ForEach(func(k string) {
			resp = append(resp, GetModuleByName(k))
		})
		return resp
	}
}

func reloadByAction(actionMds map[Action][]module.Module) {
	lock.Lock()
	defer lock.Unlock()
	if staticMode {
		return
	}
	for action, mis := range actionMds {
		//不管是哪种行为，都要删除旧模块
		for _, mi := range mis {
			if old, ok := mods[mi.Name()]; !ok {
				if action != New {
					log.Info("no active module %s, ignore", mi.Name())
				}
			} else {
				destroyMod(old)
				if action == New {
					log.Warn("register new module but old exists, destroy module %s", mi.Name())
				}
			}
		}
		//新增模块
		if action == New || action == Update {
			for _, mi := range mis {
				m := &mod{
					wg: wg.NewWaitGroup(mi.Name()),
				}
				m.mi = mi
				mods[mi.Name()] = m
				for _, tag := range mi.Tags() {
					s, ok := tags[tag]
					if !ok {
						tags[tag] = set.NewSet[string](mi.Name())
					} else {
						s.AddItem(mi.Name())
					}
				}
				mi.OnInit()
				m.wg.Add(1)
				go run(m)
				log.Info("module registered: %s", mi.Name())
			}
		}
	}
}

// staticLoadModules 静态加载，按严格的顺序加载模块
func staticLoadModules(mis []module.Module) {
	lock.Lock()
	defer lock.Unlock()
	staticMode = true
	for _, mi := range mis {
		m := &mod{
			wg: wg.NewWaitGroup(mi.Name()),
		}
		m.mi = mi
		mods[mi.Name()] = m
		for _, t := range mi.Tags() {
			if s := tags[t]; s == nil {
				tags[t] = set.NewSet[string](mi.Name())
			} else {
				s.AddItem(mi.Name())
			}
		}
		//记下启动顺序
		ordered = append(ordered, m)
		mi.OnInit()
		m.wg.Add(1)
		go run(m)
		log.Info("module registered: %s", mi.Name())
	}
	log.Info(">>>>>>>>>>>>>>>>Server Started<<<<<<<<<<<<<<<<<<<<<<<")
}

func destroyMod(m *mod) {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack(fmt.Sprintf("panic when destory module %s", m.mi.Name()), r)
		}
	}()
	m.cancelFn()
	m.wg.Wait()
	m.mi.OnDestroy()
	delete(mods, m.mi.Name())
	for _, tag := range m.mi.Tags() {
		if tags[tag] != nil {
			tags[tag].RemoveItem(m.mi.Name())
		}
	}
	log.Info("mod destroyed: %s", m.mi.Name())
}

func destroyAll() {
	lock.Lock()
	defer lock.Unlock()
	//静态模式下按着严格的逆序销毁模块
	if staticMode {
		for i := len(ordered) - 1; i >= 0; i-- {
			destroyMod(ordered[i])
		}
		return
	}
	for _, mod := range mods {
		log.Debug("destroying module %s", mod.mi.Name())
		destroyMod(mod)
	}
}

func run(m *mod) {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel
	m.mi.Run(ctx)
	m.wg.Done()
}
