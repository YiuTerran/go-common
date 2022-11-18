package transaction

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/fsm"
	"github.com/YiuTerran/go-common/sip/sip"
	"sync"
)

type TxKey = sip.TransactionKey

// Tx is a common SIP transaction
type Tx interface {
	Init() error
	Key() TxKey
	Origin() sip.Request
	// Receive receives message from transport layer.
	Receive(msg sip.Message) error
	String() string
	Transport() sip.Transport
	Terminate()
	Errors() <-chan error
	Done() <-chan bool
	Fields() log.Fields
}

type commonTx struct {
	key      TxKey
	fsm      *fsm.FSM
	fsmMu    sync.RWMutex
	origin   sip.Request
	tpl      sip.Transport
	lastResp sip.Response

	errs    chan error
	lastErr error
	done    chan bool

	fields log.Fields
}

func (tx *commonTx) String() string {
	if tx == nil {
		return "<nil>"
	}

	fields := tx.Fields().WithFields(log.Fields{
		"key": tx.key,
	})

	return fields.String()
}

func (tx *commonTx) Fields() log.Fields {
	return tx.fields
}

func (tx *commonTx) Origin() sip.Request {
	return tx.origin
}

func (tx *commonTx) Key() TxKey {
	return tx.key
}

func (tx *commonTx) Transport() sip.Transport {
	return tx.tpl
}

func (tx *commonTx) Errors() <-chan error {
	return tx.errs
}

func (tx *commonTx) Done() <-chan bool {
	return tx.done
}
