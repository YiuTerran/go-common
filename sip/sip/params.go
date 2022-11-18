package sip

import (
	"bytes"
	"fmt"
	"github.com/YiuTerran/go-common/base/structs/set"
	"sync"
)

// Params is generic list of parameters on a header.
type Params interface {
	Get(key string) (MaybeString, bool)
	Add(key string, val MaybeString) Params
	Remove(key string) Params
	Clone() Params
	Equals(params any) bool
	String() string
	Length() int
	Items() map[string]MaybeString
	Keys() []string
	Has(key string) bool
	Type() ParamType
}

type ParamType int

const (
	// UriParams uri中的参数
	UriParams ParamType = 1
	// UriHeaders uri中的header部分
	UriHeaders ParamType = 2
	// HeaderParams 一般header中的参数，如Addr/Via，使用分号分割
	HeaderParams ParamType = 3
	// AuthParams 认证参数，使用逗号分割
	AuthParams ParamType = 4
)

// IMPLEMENTATION

// 一般实现
// uri header, uri params和header三种场景都是键值对
// 但是其escape、相等判断的规则都不一样
type params struct {
	mu         sync.RWMutex
	params     map[string]MaybeString
	paramOrder []string
	pType      ParamType
}

// NewParams Create an empty set of parameters.
func NewParams(pt ParamType) *params {
	return &params{
		mu:         sync.RWMutex{},
		params:     make(map[string]MaybeString),
		paramOrder: []string{},
		pType:      pt,
	}
}

func (p *params) Type() ParamType {
	return p.pType
}

// Items Returns the entire parameter map.
func (p *params) Items() map[string]MaybeString {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.params
}

// Keys Returns a slice of keys, in order.
func (p *params) Keys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.paramOrder
}

// Get Returns the requested parameter value.
func (p *params) Get(key string) (MaybeString, bool) {
	p.mu.RLock()
	v, ok := p.params[key]
	p.mu.RUnlock()
	return v, ok
}

// Add Put a new parameter.
func (p *params) Add(key string, val MaybeString) Params {
	p.mu.Lock()
	// Add param to order list if new.
	if _, ok := p.params[key]; !ok {
		p.paramOrder = append(p.paramOrder, key)
	}

	// Set param value.
	p.params[key] = val
	p.mu.Unlock()
	// Return the params so calls can be chained.
	return p
}

func (p *params) Remove(key string) Params {
	p.mu.Lock()
	if _, ok := p.params[key]; ok {
		for k, v := range p.paramOrder {
			if v == key {
				p.paramOrder = append(p.paramOrder[:k], p.paramOrder[k+1:]...)
				break
			}
		}
		delete(p.params, key)
	}
	p.mu.Unlock()
	// Return the params so calls can be chained.
	return p
}

func (p *params) Has(key string) bool {
	p.mu.RLock()
	_, ok := p.params[key]
	p.mu.RUnlock()
	return ok
}

// Clone Copy a list of params.
func (p *params) Clone() Params {
	if p == nil {
		var dup *params
		return dup
	}

	dup := NewParams(p.pType)
	for _, key := range p.Keys() {
		if val, ok := p.Get(key); ok {
			dup.Add(key, val)
		}
	}

	return dup
}

func (p *params) String() string {
	if p == nil {
		return ""
	}
	sep := ';'
	if p.pType == UriHeaders {
		sep = '&'
	} else if p.pType == AuthParams {
		sep = ','
	}

	var buffer bytes.Buffer
	first := true

	for _, key := range p.Keys() {
		val, ok := p.Get(key)
		if !ok {
			continue
		}

		if !first {
			buffer.WriteString(fmt.Sprintf("%c", sep))
		}
		first = false
		vs, ok := val.(String)
		if p.pType == HeaderParams {
			appendSanitized(&buffer, []byte(key), tokenc)
			if ok && vs.String() != "" {
				buffer.WriteByte('=')
				appendQuoted(&buffer, []byte(vs.String()))
			}
		} else if p.pType == AuthParams {
			appendSanitized(&buffer, []byte(key), tokenc)
			if ok && vs.String() != "" {
				buffer.WriteByte('=')
				// 原样写入
				buffer.WriteString(vs.String())
			}
		} else {
			appendEscaped(&buffer, []byte(key), paramc)
			if ok && vs.String() != "" {
				buffer.WriteByte('=')
				appendEscaped(&buffer, []byte(vs.String()), paramc)
			}
		}
	}
	return buffer.String()
}

// Length returns number of params.
func (p *params) Length() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.params)
}

var (
	specialKeys   = []string{"transport", "user", "ttl", "method", "maddr"}
	specialKeySet = set.NewSet(specialKeys...)
)

// Equals Check if two maps of parameters are equal in the sense of having the same keys with the same values.
// This does not rely on any ordering of the keys of the map in memory.
// uri-params比较：
//1. 如果该参数在两边都出现，则必须严格匹配
//2. transport, user, ttl, method, maddr 如果只出现在一边，则必然不相等
//3. 除了2之外的其他参数，如果只出现在一边，则比较时忽略掉
// 其他header比较：严格相等，顺序无关
func (p *params) Equals(other any) bool {
	if other == nil {
		other = NewParams(p.pType)
	}
	q, ok := other.(*params)
	if !ok {
		return false
	}
	if p.pType != q.pType {
		return false
	}
	if p == q {
		return true
	}
	if p.Length() == 0 && q.Length() == 0 {
		return true
	}
	if p.pType == UriParams {
		ps := specialKeySet.Intersect(set.NewSet(p.Keys()...))
		qs := specialKeySet.Intersect(set.NewSet(q.Keys()...))
		//有特殊键只属于某一边
		if !ps.Equals(&qs) {
			return false
		}
		//特殊键必须严格相等
		for _, k := range ps.ToArray() {
			lhs, _ := p.Get(k)
			rhs, _ := q.Get(k)
			if !lhs.Equals(rhs) {
				return false
			}
		}
		//其他key，遍历任意一边即可
		for k, v := range p.Items() {
			if specialKeySet.Contains(k) {
				//上面已经判断过
				continue
			}
			qv, ok := q.Get(k)
			if ok {
				//两边都有，则必须相等
				if !IsStringEqual(v, qv) {
					return false
				}
			}
			//只有一边有，不影响相等
		}
	} else {
		if p.Length() != q.Length() {
			return false
		}
		for key, pVal := range p.Items() {
			qVal, ok := q.Get(key)
			if !ok {
				return false
			}
			if !IsStringEqual(pVal, qVal) {
				return false
			}
		}
	}
	return true
}

// IsParamsEqual 考虑nil场景的相等判断
func IsParamsEqual(lhs, rhs Params) bool {
	//即使长度不等，也有可能相等
	if lhs == nil {
		if rhs == nil {
			return true
		} else {
			lhs = NewParams(rhs.Type())
		}
	}
	if rhs == nil {
		if lhs == nil {
			return true
		} else {
			rhs = NewParams(lhs.Type())
		}
	}
	return lhs.Equals(rhs)
}

func cloneWithNil(params Params, pt ParamType) Params {
	if params == nil {
		return NewParams(pt)
	}
	return params.Clone()
}
