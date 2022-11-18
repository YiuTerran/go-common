package transport_test

import (
	"fmt"
	"github.com/YiuTerran/go-common/sip/transport"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	localAddr1 = fmt.Sprintf("%v:%v", transport.DefaultHost, transport.DefaultTcpPort)
	localAddr2 = fmt.Sprintf("%v:%v", transport.DefaultHost, transport.DefaultTcpPort+1)
	localAddr3 = fmt.Sprintf("%v:%v", transport.DefaultHost, transport.DefaultTcpPort+2)
)

func TestTransport(t *testing.T) {
	// setup Ginkgo
	RegisterFailHandler(Fail)
	RegisterTestingT(t)
	RunSpecs(t, "Transport Suite")
}
