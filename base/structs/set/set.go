package set

/**
  *  @author tryao
  *  @date 2022/03/18 14:15
**/

//Set 是基于map做的Set，后续标准库可能会内置
type Set[T comparable] struct {
	values map[T]struct{}
}

func NewSet[T comparable](items ...T) *Set[T] {
	var r Set[T]
	r.values = make(map[T]struct{}, len(items))
	for _, item := range items {
		r.values[item] = struct{}{}
	}
	return &r
}

func (set *Set[T]) AddItem(items ...T) *Set[T] {
	for _, item := range items {
		set.values[item] = struct{}{}
	}
	return set
}

func (set *Set[T]) RemoveItem(items ...T) *Set[T] {
	for _, item := range items {
		delete(set.values, item)
	}
	return set
}

func (set *Set[T]) Contains(item T) bool {
	_, ok := set.values[item]
	return ok
}

func (set *Set[T]) Size() int {
	return len(set.values)
}

//Union 两个set的并集
func (set *Set[T]) Union(rhs Set[T]) (r Set[T]) {
	s := make(map[T]struct{}, set.Size())
	for t, v := range set.values {
		s[t] = v
	}
	for t, v := range rhs.values {
		s[t] = v
	}
	r.values = s
	return r
}

//Intersect 两个set的交集
func (set *Set[T]) Intersect(rhs Set[T]) (r Set[T]) {
	s := make(map[T]struct{})
	for t := range set.values {
		if rhs.Contains(t) {
			s[t] = struct{}{}
		}
	}
	r.values = s
	return r
}

//Difference 两个集合的差集
func (set *Set[T]) Difference(rhs Set[T]) (r Set[T]) {
	s := make(map[T]struct{})
	for t := range set.values {
		if !rhs.Contains(t) {
			s[t] = struct{}{}
		}
	}
	r.values = s
	return r
}

//ToArray 转为数组
func (set *Set[T]) ToArray() []T {
	r := make([]T, 0, set.Size())
	for t := range set.values {
		r = append(r, t)
	}
	return r
}

//ForEach 遍历加回调
func (set *Set[T]) ForEach(f func(T)) {
	for t := range set.values {
		f(t)
	}
}
