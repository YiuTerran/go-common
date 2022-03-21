package page

/**
  *  @author tryao
  *  @date 2022/03/18 10:45
**/

const (
	MaxPageSize     = 10000
	DefaultPageSize = 20
)

type BasePage struct {
	Page int64 `json:"page"`
	Size int64 `json:"size"`
}

type PagedDTO[T any] struct {
	BasePage
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

func (p *BasePage) Normalize() {
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

func (p *BasePage) GetOffset() int64 {
	return (p.Page - 1) * p.Size
}
