package sip_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/YiuTerran/go-common/sip/parser"
	"testing"
)

/**
  *  @author tryao
  *  @date 2022/04/01 10:09
**/

func TestSipUri_Equals(t *testing.T) {
	es := [][]string{
		{"sip:%61lice@atlanta.com;transport=TCP", "sip:alice@AtLanTa.CoM;Transport=tcp"},
		{"sip:carol@chicago.com", "sip:carol@chicago.com;newparam=5", "sip:carol@chicago.com;security=on"},
		{"sip:biloxi.com?to=sip:bob%40biloxi.com", "sip:biloxi.com?TO=sip:bob%40biloxi.com"},
		{"sip:biloxi.com;transport=tcp;method=REGISTER?to=sip:bob%40biloxi.com", "sip:biloxi.com;method=REGISTER;transport=tcp?to=sip:bob%40biloxi.com"},
		{"sip:alice@atlanta.com?subject=project%20x&priority=urgent", "sip:alice@atlanta.com?priority=urgent&subject=project%20x"},
		{"sip:carol@chicago.com", "sip:carol@chicago.com;security=on"},
		{"sip:carol@chicago.com", "sip:carol@chicago.com;security=off"},
	}
	ns := [][]string{
		{"SIP:ALICE@AtLanTa.CoM;Transport=udp", "sip:alice@AtLanTa.CoM;Transport=UDP"},
		{"sip:bob@biloxi.com", "sip:bob@biloxi.com:5060"},
		{"sip:bob@biloxi.com", "sip:bob@biloxi.com;transport=udp"},
		{"sip:bob@biloxi.com", "sip:bob@biloxi.com:6000;transport=tcp"},
		{"sip:carol@chicago.com", "sip:carol@chicago.com?Subject=next%20meeting"},
		{"sip:bob@phone21.boxesbybob.com", "sip:bob@192.0.2.4"},
		{"sip:carol@chicago.com;security=on", "sip:carol@chicago.com;security=off"},
	}
	for _, vs := range es {
		u, e := parser.ParseSipUri(vs[0])
		assert.Nil(t, e)
		for i := 1; i < len(vs); i++ {
			u1, e := parser.ParseSipUri(vs[i])
			assert.Nil(t, e)
			assert.Truef(t, u.Equals(u1), "u:%+v, u1:%+v", u, u1)
		}
	}
	for _, vs := range ns {
		u, e := parser.ParseSipUri(vs[0])
		assert.Nil(t, e)
		for i := 1; i < len(vs); i++ {
			u1, e := parser.ParseSipUri(vs[i])
			assert.Nil(t, e)
			assert.False(t, u.Equals(u1), "u:%+v, u1:%+v", u, u1)
		}
	}
}
