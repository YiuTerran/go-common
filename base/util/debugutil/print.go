package debugutil

import (
	"fmt"
	"github.com/YiuTerran/go-common/base/util/debugutil/internal"
)

/**
  *  @author tryao
  *  @date 2022/03/24 08:57
**/

// Print 调试打印
func Print(v ...any) {
	_, _ = internal.P(v...)
}

// Println 带格式打印调试，所有的占位符都用%s
func Println(format string, elems ...any) {
	es := make([]any, 0, len(elems))
	for _, elem := range elems {
		es = append(es, internal.Plain(elem))
	}
	fmt.Printf(format+"\n", es...)
}
