package transport

import (
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/sip"
	"strings"
	"time"
)

const (
	//netErrRetryTime = 5 * time.Second
	sockTTL = time.Hour
)

// Protocol implements network specific features.
type Protocol interface {
	Done() <-chan struct{}
	Network() string
	Reliable() bool
	Streamed() bool
	Listen(target *Target, options ...ListenOption) error
	Send(target *Target, msg sip.Message) error
	String() string
	Fields() log.Fields
}

type ProtocolFactory func(
	network string,
	output chan<- sip.Message,
	errs chan<- error,
	cancel <-chan struct{},
	msgMapper sip.MessageMapper,
	fields log.Fields,
) (Protocol, error)

type protocol struct {
	network  string
	reliable bool
	streamed bool

	fields log.Fields
}

func (pr *protocol) Fields() log.Fields {
	return pr.fields
}

func (pr *protocol) String() string {
	if pr == nil {
		return "<nil>"
	}

	fields := pr.Fields().WithFields(log.Fields{
		"network": pr.network,
	})

	return fmt.Sprintf("transport.Protocol<%s>", fields)
}

func (pr *protocol) Network() string {
	return strings.ToUpper(pr.network)
}

func (pr *protocol) Reliable() bool {
	return pr.reliable
}

func (pr *protocol) Streamed() bool {
	return pr.streamed
}
