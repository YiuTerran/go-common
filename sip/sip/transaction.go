package sip

// rfc3261#section-17
// SIP transaction consists of a single request and any responses to
//   that request, which include zero or more provisional responses and
//   one or more final responses.  In the case of a transaction where the
//   request was an INVITE (known as an INVITE transaction), the
//   transaction also includes the ACK only if the final response was not
//   a 2xx response.  If the response was a 2xx, the ACK is not considered
//   part of the transaction.

type TransactionKey string

func (key TransactionKey) String() string {
	return string(key)
}

type Transaction interface {
	Origin() Request
	Key() TransactionKey
	String() string
	Errors() <-chan error
	Done() <-chan bool
}

type ServerTransaction interface {
	Transaction
	Respond(res Response) error
	Acks() <-chan Request
	Cancels() <-chan Request
}

type ClientTransaction interface {
	Transaction
	Responses() <-chan Response
	Cancel() error
}
