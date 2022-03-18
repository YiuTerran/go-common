package set

/**
  *  @author tryao
  *  @date 2022/03/18 14:15
**/

//Set 是基于map做的Set，后续标准库可能会内置
type Set[T comparable] struct {
	values map[T]struct{}
}

func NewSet[T comparable](items ...T) (r Set[T]) {
	r.values = make(map[T]struct{}, len(items))
	for _, item := range items {
		r.values[item] = struct{}{}
	}
	return r
}

func AddItem[T comparable](set Set[T], items ...T) Set[T] {
	for _, item := range items {
		set.values[item] = struct{}{}
	}
	return set
}

func RemoveItem[T comparable](set Set[T], items ...T) Set[T] {
	for _, item := range items {
		delete(set.values, item)
	}
	return set
}

func Contains[T comparable](set Set[T], item T) bool {
	_, ok := set.values[item]
	return ok
}

func Size[T comparable](set Set[T]) int {
	return len(set.values)
}

//Union 两个set的并集
func Union[T comparable](lhs, rhs Set[T]) (r Set[T]) {
	s := make(map[T]struct{}, Size(lhs))
	for t, v := range lhs.values {
		s[t] = v
	}
	for t, v := range rhs.values {
		s[t] = v
	}
	r.values = s
	return r
}

//Intersect 两个set的交集
func Intersect[T comparable](lhs, rhs Set[T]) (r Set[T]) {
	s := make(map[T]struct{})
	for t := range lhs.values {
		if Contains(rhs, t) {
			s[t] = struct{}{}
		}
	}
	r.values = s
	return r
}

//Difference 两个集合的差集
func Difference[T comparable](lhs, rhs Set[T]) (r Set[T]) {
	s := make(map[T]struct{})
	for t := range lhs.values {
		if !Contains(rhs, t) {
			s[t] = struct{}{}
		}
	}
	r.values = s
	return r
}

//ToArray 转为数组
func ToArray[T comparable](set Set[T]) []T {
	r := make([]T, 0, Size(set))
	for t := range set.values {
		r = append(r, t)
	}
	return r
}

//ForEach 遍历加回调
func ForEach[T comparable](set Set[T], f func(T)) {
	for t := range set.values {
		f(t)
	}
}
