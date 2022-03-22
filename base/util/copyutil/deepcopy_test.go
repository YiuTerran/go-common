package deepcopy

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

/**
  *  @author tryao
  *  @date 2022/03/22 09:53
**/
type testPointer struct {
	x int64
	Y string
}
type testCopy struct {
	private int64
	Public  string
	TP      testPointer
	tp      *testPointer
}

func (t testCopy) String() string {
	return fmt.Sprintf(`private:%d, Public:%s, TP:%+v, tp:%+v`, t.private, t.Public, t.TP, *t.tp)
}

func TestCopy(t *testing.T) {
	test := &testCopy{100, "pub", testPointer{
		x: 0,
		Y: "y",
	}, &testPointer{
		x: 1,
		Y: "y2",
	}}
	t2 := DeepCopy(test)
	assert.Equal(t, test, t2)
}
