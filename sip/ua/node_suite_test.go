package ua

/**
  *  @author tryao
  *  @date 2022/03/31 14:10
**/

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RegisterTestingT(t)
	RunSpecs(t, "GoSip Suite")
}
