package resp

/**
  *  @author tryao
  *  @date 2022/03/18 10:45
**/

const (
	MaxPageSize     = 10000
	DefaultPageSize = 20
)

type Body[T any] struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Data   T      `json:"data"`
}

type EmptyBody struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

type PageData[T any] struct {
	Page  int64 `json:"page"`
	Size  int64 `json:"size"`
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

func (p *PageData[T]) Normalize() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Size <= 0 {
		p.Size = DefaultPageSize
	}
	if p.Size > MaxPageSize {
		p.Size = MaxPageSize
	}
}

func (p *PageData[T]) Offset() int64 {
	return (p.Page - 1) * p.Size
}
