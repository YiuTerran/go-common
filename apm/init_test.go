package apm

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

/**
  *  @author tryao
  *  @date 2022/12/14 11:10
**/

var (
	testParent = []byte{0,
		0, 75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 0, 14, 71, 54,
		1, 52, 240, 103, 170, 11, 169, 2, 183,
		2, 1}
	testState = []byte{0, 3, 102, 111, 111, 16, 51, 52, 102, 48, 54, 55, 97, 97, 48, 98, 97, 57, 48, 50, 98, 55,
		0, 3, 98, 97, 114, 4, 48, 46, 50, 53}
)

func TestExtractW3CBinaryTraceParent(t *testing.T) {
	sxt, err := ExtractW3CBinaryTraceParent(testParent)
	assert.Nil(t, err)
	assert.Equal(t, "4bf92f3577b34da6a3ce929d000e4736", sxt.TraceID().String())
	assert.Equal(t, "34f067aa0ba902b7", sxt.SpanID().String())
	assert.Equal(t, true, sxt.TraceFlags().IsSampled())
}

func TestExtractW3CBinaryTraceState(t *testing.T) {
	state, err := ExtractW3CBinaryTraceState(testState)
	assert.Nil(t, err)
	assert.Equal(t, "34f067aa0ba902b7", state.Get("foo"))
	assert.Equal(t, "0.25", state.Get("bar"))
}

func TestToW3CBinary(t *testing.T) {
	sxt, _ := ExtractW3CBinaryTraceParent(testParent)
	state, _ := ExtractW3CBinaryTraceState(testState)
	sxt = sxt.WithTraceState(state)
	tp, ts := ToW3CBinary(sxt)
	assert.True(t, bytes.Equal(tp, testParent))
	assert.True(t, bytes.Equal(ts, testState))
}
