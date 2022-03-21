package errs

import "fmt"

/**
  *  @author tryao
  *  @date 2022/03/18 10:33
**/

const (
	OK              = 0
	UnknownError    = 1
	ClientError     = 2
	ThirdPartyError = 4
	ParamError      = 10
	NotAllowed      = 11
	TokenExpired    = 12
	AlreadyExist    = 13
	AuthError       = 14
	NotExist        = 15
)

var (
	statusMap = map[int]int{
		OK:           200,
		UnknownError: 500,
		NotAllowed:   403,
		AuthError:    401,
	}
)

func GetHttpStatus(status int) int {
	r, ok := statusMap[status]
	if !ok {
		return 400
	}
	return r
}

type Error struct {
	Msg    string `json:"msg"`
	Status int    `json:"status"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("status:%d, msg:%s", e.Status, e.Msg)
}
