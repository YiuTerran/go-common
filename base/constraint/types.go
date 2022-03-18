package constraint

/**
  *  @author tryao
  *  @date 2022/03/18 14:06
**/

type Signed interface {
	~int8 | ~int16 | ~int | ~int32 | ~int64
}
type UnSigned interface {
	~uint8 | ~uint16 | ~uint | ~uint32 | ~uint64
}
type Integer interface {
	Signed | UnSigned
}
type Float interface {
	~float32 | ~float64
}
type Number interface {
	Integer | Float
}
type Ordered interface {
	Integer | Float | ~string
}
