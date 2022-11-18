package transaction

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/fsm"
	"github.com/YiuTerran/go-common/base/structs/timing"
	"github.com/YiuTerran/go-common/sip/sip"
	"strings"
	"sync"
	"time"
)

// ClientTx 客户端事务
type ClientTx interface {
	Tx
	Responses() <-chan sip.Response
	Cancel() error
}

type clientTx struct {
	commonTx
	responses  chan sip.Response
	timerATime time.Duration // Current duration of timer A.
	timerA     timing.Timer
	timerB     timing.Timer
	timerDTime time.Duration // Current duration of timer D.
	timerD     timing.Timer
	timerM     timing.Timer
	reliable   bool

	mu        sync.RWMutex
	closeOnce sync.Once
}

func NewClientTx(origin sip.Request, tpl sip.Transport, fields log.Fields) (ClientTx, error) {
	origin = prepareClientRequest(origin)
	key, err := MakeClientTxKey(origin)
	if err != nil {
		return nil, err
	}

	tx := new(clientTx)
	tx.key = key
	tx.tpl = tpl
	// buffer chan - about ~10 retransmit responses
	tx.responses = make(chan sip.Response, 64)
	tx.errs = make(chan error, 64)
	tx.done = make(chan bool)
	tx.fields = origin.Fields().WithFields(log.Fields{
		"transaction_ptr": fmt.Sprintf("%p", tx),
		"transaction_key": tx.key,
	}).WithPrefix("transaction.ClientTx").
		WithFields(fields)
	tx.origin = origin.WithFields(log.Fields{
		"transaction_ptr": fmt.Sprintf("%p", tx),
		"transaction_key": tx.key,
	}).(sip.Request)
	tx.reliable = tx.tpl.IsReliable(origin.Transport())

	return tx, nil
}

func prepareClientRequest(origin sip.Request) sip.Request {
	if viaHop, ok := origin.ViaHop(); ok {
		if viaHop.Params == nil {
			viaHop.Params = sip.NewParams(sip.HeaderParams)
		}
		if !viaHop.Params.Has("branch") {
			viaHop.Params.Add("branch", sip.String{Str: sip.GenerateBranch()})
		}
	} else {
		viaHop = &sip.ViaHop{
			ProtocolName:    "SIP",
			ProtocolVersion: "2.0",
			Transport:       origin.Transport(),
			Host:            sip.DefaultHost,
			Port:            lo.ToPtr(sip.DefaultPort(strings.ToLower(origin.Transport()))),
			Params: sip.NewParams(sip.HeaderParams).
				Add("branch", sip.String{Str: sip.GenerateBranch()}),
		}

		origin.PrependHeader(sip.ViaHeader{viaHop})
	}

	return origin
}

func (tx *clientTx) Fields() log.Fields {
	return tx.commonTx.Fields()
}

func (tx *clientTx) Init() error {
	tx.initFSM()

	if err := tx.tpl.Send(tx.Origin()); err != nil {
		tx.mu.Lock()
		tx.lastErr = err
		tx.mu.Unlock()

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(clientInputTransportErr); err != nil {
			tx.Fields().Error("spin FSM to client_input_transport_err failed: %s", err)
		}
		tx.fsmMu.RUnlock()

		return err
	}

	if tx.reliable {
		tx.mu.Lock()
		tx.timerDTime = 0
		tx.mu.Unlock()
	} else {
		// RFC 3261 - 17.1.1.2.
		// If unreliable transport is being used, the client transaction MUST start timer A with a value of T1.
		// If reliable transport is being used, the client transaction SHOULD NOT start timer A
		// (Timer A controls request retransmissions).
		// Timer A - retransmission
		tx.mu.Lock()
		tx.timerATime = TimerA

		tx.timerA = timing.AfterFunc(tx.timerATime, func() {
			select {
			case <-tx.done:
				return
			default:
			}

			tx.Fields().Debug("timer_a fired")

			tx.fsmMu.RLock()
			if err := tx.fsm.Spin(clientInputTimerA); err != nil {
				tx.Fields().Error("spin FSM to client_input_timer_a failed: %s", err)
			}
			tx.fsmMu.RUnlock()
		})
		if tx.Origin().IsInvite() {
			// Timer D is set to 32 seconds for unreliable transports
			tx.timerDTime = TimerD
		} else {
			// non-invite, use timerD as timerK
			tx.timerDTime = TimerK
		}
		tx.mu.Unlock()
	}

	// Timer B - timeout
	tx.mu.Lock()
	tx.timerB = timing.AfterFunc(TimerB, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_b fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(clientInputTimerB); err != nil {
			tx.Fields().Error("spin FSM to client_input_timer_b failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})
	tx.mu.Unlock()

	tx.mu.RLock()
	err := tx.lastErr
	tx.mu.RUnlock()

	return err
}

func (tx *clientTx) Receive(msg sip.Message) error {
	res, ok := msg.(sip.Response)
	if !ok {
		return &sip.UnexpectedMessageError{
			Err: fmt.Errorf("%s recevied unexpected %s", tx, msg.Short()),
			Msg: msg.String(),
		}
	}

	res = res.WithFields(log.Fields{
		"request_id": tx.origin.MessageID(),
	}).(sip.Response)

	var input fsm.Input
	if res.IsCancel() {
		input = clientInputCanceled
	} else {
		tx.mu.Lock()
		tx.lastResp = res
		tx.mu.Unlock()

		switch {
		case res.IsProvisional():
			input = clientInput1xx
		case res.IsSuccess():
			input = clientInput2xx
		default:
			input = clientInput300Plus
		}
	}

	tx.fsmMu.RLock()
	defer tx.fsmMu.RUnlock()

	return tx.fsm.Spin(input)
}

func (tx *clientTx) Responses() <-chan sip.Response {
	return tx.responses
}

func (tx *clientTx) Cancel() error {
	tx.fsmMu.RLock()
	defer tx.fsmMu.RUnlock()

	return tx.fsm.Spin(clientInputCancel)
}

func (tx *clientTx) Terminate() {
	select {
	case <-tx.done:
		return
	default:
	}

	tx.delete()
}

func (tx *clientTx) cancel() {
	if !tx.Origin().IsInvite() {
		return
	}

	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	cancelRequest := sip.NewCancelRequest("", tx.Origin(), log.Fields{
		"sent_at": time.Now(),
	})
	if err := tx.tpl.Send(cancelRequest); err != nil {
		var lastRespStr string
		if lastResp != nil {
			lastRespStr = lastResp.Short()
		}
		tx.Fields().WithFields(log.Fields{
			"invite_request":  tx.Origin().Short(),
			"invite_response": lastRespStr,
			"cancel_request":  cancelRequest.Short(),
		}).Error("send CANCEL request failed: %s", err)

		tx.mu.Lock()
		tx.lastErr = err
		tx.mu.Unlock()

		go func() {
			tx.fsmMu.RLock()
			if err := tx.fsm.Spin(clientInputTransportErr); err != nil {
				tx.Fields().Error("spin FSM to client_input_transport_err failed: %s", err)
			}
			tx.fsmMu.RUnlock()
		}()
	}
}

func (tx *clientTx) ack() {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	ack := sip.NewAckRequest("", tx.Origin(), lastResp, "", log.Fields{
		"sent_at": time.Now(),
	})
	err := tx.tpl.Send(ack)
	if err != nil {
		tx.Fields().WithFields(log.Fields{
			"invite_request":  tx.Origin().Short(),
			"invite_response": lastResp.Short(),
			"ack_request":     ack.Short(),
		}).Error("send ACK request failed: %s", err)

		tx.mu.Lock()
		tx.lastErr = err
		tx.mu.Unlock()

		go func() {
			tx.fsmMu.RLock()
			if err := tx.fsm.Spin(clientInputTransportErr); err != nil {
				tx.Fields().Error("spin FSM to client_input_transport_err failed: %s", err)
			}
			tx.fsmMu.RUnlock()
		}()
	}
}

// FSM States
const (
	clientStateCalling = iota
	clientStateProceeding
	clientStateCompleted
	clientStateAccepted
	clientStateTerminated
)

// FSM Inputs
const (
	clientInput1xx fsm.Input = iota
	clientInput2xx
	clientInput300Plus
	clientInputTimerA
	clientInputTimerB
	clientInputTimerD
	clientInputTimerM
	clientInputTransportErr
	clientInputDelete
	clientInputCancel
	clientInputCanceled
)

// Initialises the correct kind of FSM based on request method.
func (tx *clientTx) initFSM() {
	if tx.Origin().IsInvite() {
		tx.initInviteFSM()
	} else {
		tx.initNonInviteFSM()
	}
}

//                                |INVITE from TU
//             Timer A fires     |INVITE sent
//             Reset A,          V                      Timer B fires
//             INVITE sent +-----------+                or Transport Err.
//               +---------|           |---------------+inform TU
//               |         |  Calling  |               |
//               +-------->|           |-------------->|
//                         +-----------+ 2xx           |
//                            |  |       2xx to TU     |
//                            |  |1xx                  |
//    300-699 +---------------+  |1xx to TU            |
//   ACK sent |                  |                     |
//resp. to TU |  1xx             V                     |
//            |  1xx to TU  -----------+               |
//            |  +---------|           |               |
//            |  |         |Proceeding |-------------->|
//            |  +-------->|           | 2xx           |
//            |            +-----------+ 2xx to TU     |
//            |       300-699    |                     |
//            |       ACK sent,  |                     |
//            |       resp. to TU|                     |
//            |                  |                     |      NOTE:
//            |  300-699         V                     |
//            |  ACK sent  +-----------+Transport Err. |  transitions
//            |  +---------|           |Inform TU      |  labeled with
//            |  |         | Completed |-------------->|  the event
//            |  +-------->|           |               |  over the action
//            |            +-----------+               |  to take
//            |              ^   |                     |
//            |              |   | Timer D fires       |
//            +--------------+   | -                   |
//                               |                     |
//                               V                     |
//                         +-----------+               |
//                         |           |               |
//                         | Terminated|<--------------+
//                         |           |
//                         +-----------+

func (tx *clientTx) initInviteFSM() {
	// Define States
	// Calling
	clientStateDefCalling := fsm.State{
		Index: clientStateCalling,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateProceeding, tx.actInviteProceeding},
			clientInput2xx:          {clientStateAccepted, tx.actPassUpAccept},
			clientInput300Plus:      {clientStateCompleted, tx.actInviteFinal},
			clientInputCancel:       {clientStateCalling, tx.actCancel},
			clientInputCanceled:     {clientStateCalling, tx.actInviteCanceled},
			clientInputTimerA:       {clientStateCalling, tx.actInviteResend},
			clientInputTimerB:       {clientStateTerminated, tx.actTimeout},
			clientInputTransportErr: {clientStateTerminated, tx.actTransErr},
		},
	}

	// Proceeding
	clientStateDefProceeding := fsm.State{
		Index: clientStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateProceeding, tx.actPassUp},
			clientInput2xx:          {clientStateAccepted, tx.actPassUpAccept},
			clientInput300Plus:      {clientStateCompleted, tx.actInviteFinal},
			clientInputCancel:       {clientStateProceeding, tx.actCancelTimeout},
			clientInputCanceled:     {clientStateProceeding, tx.actInviteCanceled},
			clientInputTimerA:       {clientStateProceeding, fsm.NoAction},
			clientInputTimerB:       {clientStateTerminated, tx.actTimeout},
			clientInputTransportErr: {clientStateTerminated, tx.actTransErr},
		},
	}

	// Completed
	clientStateDefCompleted := fsm.State{
		Index: clientStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateCompleted, fsm.NoAction},
			clientInput2xx:          {clientStateCompleted, fsm.NoAction},
			clientInput300Plus:      {clientStateCompleted, tx.actAck},
			clientInputCancel:       {clientStateCompleted, fsm.NoAction},
			clientInputCanceled:     {clientStateCompleted, fsm.NoAction},
			clientInputTransportErr: {clientStateTerminated, tx.actTransErr},
			clientInputTimerA:       {clientStateCompleted, fsm.NoAction},
			clientInputTimerB:       {clientStateCompleted, fsm.NoAction},
			clientInputTimerD:       {clientStateTerminated, tx.actDelete},
		},
	}

	clientStateDefAccepted := fsm.State{
		Index: clientStateAccepted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:      {clientStateAccepted, fsm.NoAction},
			clientInput2xx:      {clientStateAccepted, tx.actPassUp},
			clientInput300Plus:  {clientStateAccepted, fsm.NoAction},
			clientInputCancel:   {clientStateAccepted, fsm.NoAction},
			clientInputCanceled: {clientStateAccepted, fsm.NoAction},
			clientInputTransportErr: {clientStateAccepted, func() fsm.Input {
				tx.actTransErr()
				return fsm.NoInput
			}},
			clientInputTimerA: {clientStateAccepted, fsm.NoAction},
			clientInputTimerB: {clientStateAccepted, fsm.NoAction},
			clientInputTimerM: {clientStateTerminated, tx.actDelete},
		},
	}

	// Terminated
	clientStateDefTerminated := fsm.State{
		Index: clientStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateTerminated, fsm.NoAction},
			clientInput2xx:          {clientStateTerminated, fsm.NoAction},
			clientInput300Plus:      {clientStateTerminated, fsm.NoAction},
			clientInputCancel:       {clientStateTerminated, fsm.NoAction},
			clientInputCanceled:     {clientStateTerminated, fsm.NoAction},
			clientInputTimerA:       {clientStateTerminated, fsm.NoAction},
			clientInputTimerB:       {clientStateTerminated, fsm.NoAction},
			clientInputTimerD:       {clientStateTerminated, fsm.NoAction},
			clientInputTimerM:       {clientStateTerminated, fsm.NoAction},
			clientInputDelete:       {clientStateTerminated, tx.actDelete},
			clientInputTransportErr: {clientStateTerminated, fsm.NoAction},
		},
	}

	fsm_, err := fsm.Define(
		clientStateDefCalling,
		clientStateDefProceeding,
		clientStateDefCompleted,
		clientStateDefAccepted,
		clientStateDefTerminated,
	)

	if err != nil {
		tx.Fields().Error("define INVITE transaction FSM failed: %s", err)

		return
	}

	tx.fsmMu.Lock()
	tx.fsm = fsm_
	tx.fsmMu.Unlock()
}

//                                    |Request from TU
//                                   |send request
//               Timer E             V
//               send request  +-----------+
//                   +---------|           |-------------------+
//                   |         |  Trying   |  Timer F          |
//                   +-------->|           |  or Transport Err.|
//                             +-----------+  inform TU        |
//                200-699         |  |                         |
//                resp. to TU     |  |1xx                      |
//                +---------------+  |resp. to TU              |
//                |                  |                         |
//                |   Timer E        V       Timer F           |
//                |   send req +-----------+ or Transport Err. |
//                |  +---------|           | inform TU         |
//                |  |         |Proceeding |------------------>|
//                |  +-------->|           |-----+             |
//                |            +-----------+     |1xx          |
//                |              |      ^        |resp to TU   |
//                | 200-699      |      +--------+             |
//                | resp. to TU  |                             |
//                |              |                             |
//                |              V                             |
//                |            +-----------+                   |
//                |            |           |                   |
//                |            | Completed |                   |
//                |            |           |                   |
//                |            +-----------+                   |
//                |              ^   |                         |
//                |              |   | Timer K                 |
//                +--------------+   | -                       |
//                                   |                         |
//                                   V                         |
//             NOTE:           +-----------+                   |
//                             |           |                   |
//         transitions         | Terminated|<------------------+
//         labeled with        |           |
//         the event           +-----------+
//         over the action
//         to take

func (tx *clientTx) initNonInviteFSM() {
	// Define States
	// "Trying"
	// 此时timerA相当于rfc中的timerE, timerB相当于文档中的timerF, timerD相当于timerK
	clientStateDefCalling := fsm.State{
		Index: clientStateCalling,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateProceeding, tx.actPassUp},
			clientInput2xx:          {clientStateCompleted, tx.actNonInviteFinal},
			clientInput300Plus:      {clientStateCompleted, tx.actNonInviteFinal},
			clientInputTimerA:       {clientStateCalling, tx.actNonInviteResend},
			clientInputTimerB:       {clientStateTerminated, tx.actTimeout},
			clientInputTransportErr: {clientStateTerminated, tx.actTransErr},
			clientInputCancel:       {clientStateCalling, fsm.NoAction},
		},
	}

	// Proceeding
	clientStateDefProceeding := fsm.State{
		Index: clientStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateProceeding, tx.actPassUp},
			clientInput2xx:          {clientStateCompleted, tx.actNonInviteFinal},
			clientInput300Plus:      {clientStateCompleted, tx.actNonInviteFinal},
			clientInputTimerA:       {clientStateProceeding, tx.actNonInviteResend},
			clientInputTimerB:       {clientStateTerminated, tx.actTimeout},
			clientInputTransportErr: {clientStateTerminated, tx.actTransErr},
			clientInputCancel:       {clientStateProceeding, fsm.NoAction},
		},
	}

	// Completed
	clientStateDefCompleted := fsm.State{
		Index: clientStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:     {clientStateCompleted, fsm.NoAction},
			clientInput2xx:     {clientStateCompleted, fsm.NoAction},
			clientInput300Plus: {clientStateCompleted, fsm.NoAction},
			clientInputTimerA:  {clientStateCompleted, fsm.NoAction},
			clientInputTimerB:  {clientStateCompleted, fsm.NoAction},
			clientInputTimerD:  {clientStateTerminated, tx.actDelete},
			clientInputCancel:  {clientStateCompleted, fsm.NoAction},
		},
	}

	// Terminated
	clientStateDefTerminated := fsm.State{
		Index: clientStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:     {clientStateTerminated, fsm.NoAction},
			clientInput2xx:     {clientStateTerminated, fsm.NoAction},
			clientInput300Plus: {clientStateTerminated, fsm.NoAction},
			clientInputTimerA:  {clientStateTerminated, fsm.NoAction},
			clientInputTimerB:  {clientStateTerminated, fsm.NoAction},
			clientInputTimerD:  {clientStateTerminated, fsm.NoAction},
			clientInputDelete:  {clientStateTerminated, tx.actDelete},
			clientInputCancel:  {clientStateTerminated, fsm.NoAction},
		},
	}

	fsm_, err := fsm.Define(
		clientStateDefCalling,
		clientStateDefProceeding,
		clientStateDefCompleted,
		clientStateDefTerminated,
	)

	if err != nil {
		tx.Fields().Error("define non-INVITE transaction FSM failed: %s", err)

		return
	}

	tx.fsmMu.Lock()
	tx.fsm = fsm_
	tx.fsmMu.Unlock()
}

func (tx *clientTx) resend() {
	select {
	case <-tx.done:
		return
	default:
	}

	tx.Fields().Debug("resend origin request")

	err := tx.tpl.Send(tx.Origin())

	tx.mu.Lock()
	tx.lastErr = err
	tx.mu.Unlock()

	if err != nil {
		go func() {
			tx.fsmMu.RLock()
			if err := tx.fsm.Spin(clientInputTransportErr); err != nil {
				tx.Fields().Error("spin FSM to client_input_transport_err failed: %s", err)
			}
			tx.fsmMu.RUnlock()
		}()
	}
}

func (tx *clientTx) passUp() {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp != nil {
		select {
		case <-tx.done:
		case tx.responses <- lastResp:
		}
	}
}

func (tx *clientTx) transportErr() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("sip transportErr", r)
		}
	}()

	tx.mu.RLock()
	err := tx.lastErr
	tx.mu.RUnlock()

	err = &TxTransportError{
		fmt.Errorf("transaction failed to send %s: %w", tx.origin.Short(), err),
		tx.Key(),
		fmt.Sprintf("%p", tx),
	}

	select {
	case <-tx.done:
	case tx.errs <- err:
	}
}

func (tx *clientTx) timeoutErr() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("sip TimeoutErr", r)
		}
	}()

	err := &TxTimeoutError{
		fmt.Errorf("transaction timed out"),
		tx.Key(),
		fmt.Sprintf("%p", tx),
	}

	select {
	case <-tx.done:
	case tx.errs <- err:
	}
}

func (tx *clientTx) delete() {
	select {
	case <-tx.done:
		return
	default:
	}
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("sip delete tx", r)
		}
	}()

	tx.closeOnce.Do(func() {
		tx.mu.Lock()

		close(tx.done)
		close(tx.responses)
		close(tx.errs)

		tx.mu.Unlock()
		tx.Fields().Debug("transaction done")
	})

	time.Sleep(time.Microsecond)

	tx.mu.Lock()
	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerB != nil {
		tx.timerB.Stop()
		tx.timerB = nil
	}
	if tx.timerD != nil {
		tx.timerD.Stop()
		tx.timerD = nil
	}
	tx.mu.Unlock()
}

// Define actions
func (tx *clientTx) actInviteResend() fsm.Input {
	tx.Fields().Debug("act_invite_resend")

	tx.mu.Lock()

	tx.timerATime *= 2
	tx.timerA.Reset(tx.timerATime)

	tx.mu.Unlock()

	tx.resend()

	return fsm.NoInput
}

func (tx *clientTx) actInviteCanceled() fsm.Input {
	tx.Fields().Debug("act_invite_canceled")

	// nothing to do here for now

	return fsm.NoInput
}

func (tx *clientTx) actNonInviteResend() fsm.Input {
	tx.Fields().Debug("act_non_invite_resend")

	tx.mu.Lock()

	tx.timerATime *= 2
	// For non-INVITE, cap timer A at T2 seconds.
	if tx.timerATime > T2 {
		tx.timerATime = T2
	}
	tx.timerA.Reset(tx.timerATime)

	tx.mu.Unlock()

	tx.resend()

	return fsm.NoInput
}

func (tx *clientTx) actPassUp() fsm.Input {
	tx.passUp()
	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *clientTx) actInviteProceeding() fsm.Input {
	tx.Fields().Debug("act_invite_proceeding")
	tx.passUp()
	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerB != nil {
		tx.timerB.Stop()
		tx.timerB = nil
	}

	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *clientTx) actInviteFinal() fsm.Input {
	tx.Fields().Debug("act_invite_final")

	tx.ack()
	tx.passUp()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerB != nil {
		tx.timerB.Stop()
		tx.timerB = nil
	}

	tx.timerD = timing.AfterFunc(tx.timerDTime, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_d fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(clientInputTimerD); err != nil {
			tx.Fields().Debug("spin FSM to client_input_timer_d failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})

	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *clientTx) actNonInviteFinal() fsm.Input {
	tx.Fields().Debug("act_non_invite_final")

	tx.passUp()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerB != nil {
		tx.timerB.Stop()
		tx.timerB = nil
	}

	tx.timerD = timing.AfterFunc(tx.timerDTime, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_d fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(clientInputTimerD); err != nil {
			tx.Fields().Error("spin FSM to client_input_timer_d failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})

	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *clientTx) actCancel() fsm.Input {
	tx.Fields().Debug("act_cancel")

	tx.cancel()

	return fsm.NoInput
}

func (tx *clientTx) actCancelTimeout() fsm.Input {
	tx.Fields().Debug("act_cancel")

	tx.cancel()

	tx.mu.Lock()
	if tx.timerB != nil {
		tx.timerB.Stop()
	}
	tx.timerB = timing.AfterFunc(TimerB, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_b fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(clientInputTimerB); err != nil {
			tx.Fields().Error("spin FSM to client_input_timer_b failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})
	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *clientTx) actAck() fsm.Input {
	tx.Fields().Debug("act_ack")

	tx.ack()

	return fsm.NoInput
}

func (tx *clientTx) actTransErr() fsm.Input {
	tx.Fields().Debug("act_trans_err")

	tx.transportErr()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return clientInputDelete
}

func (tx *clientTx) actTimeout() fsm.Input {
	tx.Fields().Debug("act_timeout")

	tx.timeoutErr()
	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return clientInputDelete
}

func (tx *clientTx) actPassUpDelete() fsm.Input {
	tx.passUp()
	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return clientInputDelete
}

func (tx *clientTx) actPassUpAccept() fsm.Input {
	tx.passUp()
	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerB != nil {
		tx.timerB.Stop()
		tx.timerB = nil
	}

	tx.timerM = timing.AfterFunc(TimerM, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_m fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(clientInputTimerM); err != nil {
			tx.Fields().Error("spin FSM to client_input_timer_m failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})
	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *clientTx) actDelete() fsm.Input {
	tx.delete()
	return fsm.NoInput
}
