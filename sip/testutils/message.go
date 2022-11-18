package testutils

import (
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/parser"
	"github.com/YiuTerran/go-common/sip/sip"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func Message(rawMsg []string) sip.Message {
	msg, err := parser.ParseMessage([]byte(strings.Join(rawMsg, "\r\n")), log.Fields{})
	Expect(err).ToNot(HaveOccurred())
	return msg
}

func Request(rawMsg []string) sip.Request {
	msg := Message(rawMsg)
	switch msg := msg.(type) {
	case sip.Request:
		return msg
	case sip.Response:
		Fail(fmt.Sprintf("%s is not a request", msg.Short()))
	default:
		Fail(fmt.Sprintf("%s is not a request", msg.Short()))
	}
	return nil
}

func Response(rawMsg []string) sip.Response {
	msg := Message(rawMsg)
	switch msg := msg.(type) {
	case sip.Response:
		return msg
	case sip.Request:
		Fail(fmt.Sprintf("%s is not a response", msg.Short()))
	default:
		Fail(fmt.Sprintf("%s is not a response", msg.Short()))
	}
	return nil
}
