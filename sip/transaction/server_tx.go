package transaction

import (
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/fsm"
	"github.com/YiuTerran/go-common/base/structs/timing"
	"github.com/YiuTerran/go-common/sip/sip"
	"sync"
	"time"
)

type ServerTx interface {
	Tx
	Respond(res sip.Response) error
	Acks() <-chan sip.Request
	Cancels() <-chan sip.Request
}

type serverTx struct {
	commonTx
	lastAck    sip.Request
	lastCancel sip.Request
	acks       chan sip.Request
	cancels    chan sip.Request
	timerG     timing.Timer
	timerGTime time.Duration
	timerH     timing.Timer
	timerI     timing.Timer
	timerITime time.Duration
	timerJ     timing.Timer
	timer1xx   timing.Timer
	timerL     timing.Timer
	reliable   bool

	mu        sync.RWMutex
	closeOnce sync.Once
}

func NewServerTx(origin sip.Request, tpl sip.Transport, fields log.Fields) (ServerTx, error) {
	key, err := MakeServerTxKey(origin)
	if err != nil {
		return nil, err
	}

	tx := new(serverTx)
	tx.key = key
	tx.tpl = tpl
	// about ~10 retransmits
	tx.acks = make(chan sip.Request, 64)
	tx.cancels = make(chan sip.Request, 64)
	tx.errs = make(chan error, 64)
	tx.done = make(chan bool)
	tx.fields = fields.
		WithPrefix("transaction.ServerTx").
		WithFields(
			origin.Fields().WithFields(log.Fields{
				"transaction_ptr": fmt.Sprintf("%p", tx),
				"transaction_key": tx.key,
			}),
		)
	tx.origin = origin.WithFields(log.Fields{
		"transaction_ptr": fmt.Sprintf("%p", tx),
		"transaction_key": tx.key,
	}).(sip.Request)
	tx.reliable = tx.tpl.IsReliable(origin.Transport())

	return tx, nil
}

func (tx *serverTx) Init() error {
	tx.initFSM()

	tx.mu.Lock()

	if tx.reliable {
		tx.timerITime = 0
	} else {
		tx.timerGTime = TimerG
		tx.timerITime = TimerI
	}

	tx.mu.Unlock()

	// RFC 3261 - 17.2.1
	if tx.Origin().IsInvite() {
		tx.mu.Lock()
		tx.timer1xx = timing.AfterFunc(Timer1xx, func() {
			select {
			case <-tx.done:
				return
			default:
			}

			tx.Fields().Debug("timer_1xx fired")
			if err := tx.Respond(
				sip.NewResponseFromRequest(
					"",
					tx.Origin(),
					100,
					"Trying",
					"",
				),
			); err != nil {
				tx.Fields().Error("send '100 Trying' response failed: %s", err)
			}
		})
		tx.mu.Unlock()
	}

	return nil
}

func (tx *serverTx) Receive(msg sip.Message) error {
	req, ok := msg.(sip.Request)
	if !ok {
		return &sip.UnexpectedMessageError{
			Err: fmt.Errorf("%s recevied unexpected %s", tx, msg),
			Msg: req.String(),
		}
	}

	tx.mu.Lock()
	if tx.timer1xx != nil {
		tx.timer1xx.Stop()
		tx.timer1xx = nil
	}
	tx.mu.Unlock()

	var input = fsm.NoInput
	switch {
	case req.Method() == tx.Origin().Method():
		input = serverInputRequest
	case req.IsAck(): // ACK for non-2xx response
		// 兼容部分错误的uac
		if tx.fsm.Current() == serverStateAccepted {
			return &sip.UnexpectedMessageError{
				Err: errors.New("ack for non 2xx, but current state is from 2xx response"), Msg: req.String(),
			}
		}
		input = serverInputAck
		tx.mu.Lock()
		tx.lastAck = req
		tx.mu.Unlock()
	case req.IsCancel():
		input = serverInputCancel
		tx.mu.Lock()
		tx.lastCancel = req
		tx.mu.Unlock()
	default:
		return &sip.UnexpectedMessageError{
			Err: fmt.Errorf("invalid %s correlated to %s", msg, tx),
			Msg: req.String(),
		}
	}

	tx.fsmMu.RLock()
	defer tx.fsmMu.RUnlock()

	return tx.fsm.Spin(input)
}

func (tx *serverTx) Respond(res sip.Response) error {
	if res.IsCancel() {
		_ = tx.tpl.Send(res)
		return nil
	}

	tx.mu.Lock()
	tx.lastResp = res

	if tx.timer1xx != nil {
		tx.timer1xx.Stop()
		tx.timer1xx = nil
	}
	tx.mu.Unlock()

	var input fsm.Input
	switch {
	case res.IsProvisional():
		input = serverInputUser1xx
	case res.IsSuccess():
		input = serverInputUser2xx
	default:
		input = serverInputUser300Plus
	}

	tx.fsmMu.RLock()
	defer tx.fsmMu.RUnlock()

	return tx.fsm.Spin(input)
}

func (tx *serverTx) Acks() <-chan sip.Request {
	return tx.acks
}

func (tx *serverTx) Cancels() <-chan sip.Request {
	return tx.cancels
}

func (tx *serverTx) Terminate() {
	select {
	case <-tx.done:
		return
	default:
	}

	tx.delete()
}

// FSM States
const (
	serverStateTrying = iota
	serverStateProceeding
	serverStateCompleted
	serverStateConfirmed
	serverStateAccepted
	serverStateTerminated
)

// FSM Inputs
const (
	serverInputRequest fsm.Input = iota
	serverInputAck
	serverInputCancel
	serverInputUser1xx
	serverInputUser2xx
	serverInputUser300Plus
	serverInputTimerG
	serverInputTimerH
	serverInputTimerI
	serverInputTimerJ
	serverInputTimerL
	serverInputTransportErr
	serverInputDelete
)

// Choose the right FSM init function depending on request method.
func (tx *serverTx) initFSM() {
	if tx.Origin().IsInvite() {
		tx.initInviteFSM()
	} else {
		tx.initNonInviteFSM()
	}
}

//                               |INVITE
//                               |pass INV to TU
//            INVITE             V send 100 if TU won't in 200ms
//            send response+-----------+
//                +--------|           |--------+101-199 from TU
//                |        | Proceeding|        |send response
//                +------->|           |<-------+
//                         |           |          Transport Err.
//                         |           |          Inform TU
//                         |           |--------------->+
//                         +-----------+                |
//            300-699 from TU |     |2xx from TU        |
//            send response   |     |send response      |
//                            |     +------------------>+
//                            |                         |
//            INVITE          V          Timer G fires  |
//            send response+-----------+ send response  |
//                +--------|           |--------+       |
//                |        | Completed |        |       |
//                +------->|           |<-------+       |
//                         +-----------+                |
//                            |     |                   |
//                        ACK |     |                   |
//                        -   |     +------------------>+
//                            |        Timer H fires    |
//                            V        or Transport Err.|
//                         +-----------+  Inform TU     |
//                         |           |                |
//                         | Confirmed |                |
//                         |           |                |
//                         +-----------+                |
//                               |                      |
//                               |Timer I fires         |
//                               |-                     |
//                               |                      |
//                               V                      |
//                         +-----------+                |
//                         |           |                |
//                         | Terminated|<---------------+
//                         |           |
//                         +-----------+

func (tx *serverTx) initInviteFSM() {
	// Define States
	tx.Fields().Debug("initialising INVITE transaction FSM")

	// Proceeding
	serverStateDefProceeding := fsm.State{
		Index: serverStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateProceeding, tx.actRespond},
			serverInputCancel:       {serverStateProceeding, tx.actCancel},
			serverInputUser1xx:      {serverStateProceeding, tx.actRespond},
			serverInputUser2xx:      {serverStateAccepted, tx.actRespondAccept},
			serverInputUser300Plus:  {serverStateCompleted, tx.actRespondComplete},
			serverInputTransportErr: {serverStateTerminated, tx.actTransErr},
		},
	}

	// Completed
	serverStateDefCompleted := fsm.State{
		Index: serverStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateCompleted, tx.actRespond},
			serverInputAck:          {serverStateConfirmed, tx.actConfirm},
			serverInputCancel:       {serverStateCompleted, fsm.NoAction},
			serverInputUser1xx:      {serverStateCompleted, fsm.NoAction},
			serverInputUser2xx:      {serverStateCompleted, fsm.NoAction},
			serverInputUser300Plus:  {serverStateCompleted, fsm.NoAction},
			serverInputTimerG:       {serverStateCompleted, tx.actRespondComplete},
			serverInputTimerH:       {serverStateTerminated, tx.actDelete},
			serverInputTransportErr: {serverStateTerminated, tx.actTransErr},
		},
	}

	// Confirmed
	serverStateDefConfirmed := fsm.State{
		Index: serverStateConfirmed,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:     {serverStateConfirmed, fsm.NoAction},
			serverInputAck:         {serverStateConfirmed, fsm.NoAction},
			serverInputCancel:      {serverStateConfirmed, fsm.NoAction},
			serverInputUser1xx:     {serverStateConfirmed, fsm.NoAction},
			serverInputUser2xx:     {serverStateConfirmed, fsm.NoAction},
			serverInputUser300Plus: {serverStateConfirmed, fsm.NoAction},
			serverInputTimerI:      {serverStateTerminated, tx.actDelete},
			serverInputTimerG:      {serverStateConfirmed, fsm.NoAction},
			serverInputTimerH:      {serverStateConfirmed, fsm.NoAction},
		},
	}

	serverStateDefAccepted := fsm.State{
		Index: serverStateAccepted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateAccepted, fsm.NoAction},
			serverInputAck:          {serverStateAccepted, tx.actPassUpAck},
			serverInputCancel:       {serverStateAccepted, fsm.NoAction},
			serverInputUser1xx:      {serverStateAccepted, fsm.NoAction},
			serverInputUser2xx:      {serverStateAccepted, tx.actRespond},
			serverInputUser300Plus:  {serverStateAccepted, fsm.NoAction},
			serverInputTransportErr: {serverStateAccepted, fsm.NoAction},
			serverInputTimerL:       {serverStateTerminated, tx.actDelete},
		},
	}

	// Terminated
	serverStateDefTerminated := fsm.State{
		Index: serverStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:     {serverStateTerminated, fsm.NoAction},
			serverInputAck:         {serverStateTerminated, fsm.NoAction},
			serverInputCancel:      {serverStateTerminated, fsm.NoAction},
			serverInputUser1xx:     {serverStateTerminated, fsm.NoAction},
			serverInputUser2xx:     {serverStateTerminated, fsm.NoAction},
			serverInputUser300Plus: {serverStateTerminated, fsm.NoAction},
			serverInputDelete:      {serverStateTerminated, tx.actDelete},
			serverInputTimerI:      {serverStateTerminated, fsm.NoAction},
			serverInputTimerL:      {serverStateTerminated, fsm.NoAction},
		},
	}

	// Define FSM
	fsm_, err := fsm.Define(
		serverStateDefProceeding,
		serverStateDefCompleted,
		serverStateDefConfirmed,
		serverStateDefAccepted,
		serverStateDefTerminated,
	)
	if err != nil {
		tx.Fields().Error("define INVITE transaction FSM failed: %s", err)

		return
	}

	tx.fsmMu.Lock()
	tx.fsm = fsm_
	tx.fsmMu.Unlock()
}

//                                   |Request received
//                                  |pass to TU
//                                  V
//                            +-----------+
//                            |           |
//                            | Trying    |-------------+
//                            |           |             |
//                            +-----------+             |200-699 from TU
//                                  |                   |send response
//                                  |1xx from TU        |
//                                  |send response      |
//                                  |                   |
//               Request            V      1xx from TU  |
//               send response+-----------+send response|
//                   +--------|           |--------+    |
//                   |        | Proceeding|        |    |
//                   +------->|           |<-------+    |
//            +<--------------|           |             |
//            |Transport Err  +-----------+             |
//            |Inform TU            |                   |
//            |                     |                   |
//            |                     |200-699 from TU    |
//            |                     |send response      |
//            |  Request            V                   |
//            |  send response+-----------+             |
//            |      +--------|           |             |
//            |      |        | Completed |<------------+
//            |      +------->|           |
//            +<--------------|           |
//            |Transport Err  +-----------+
//            |Inform TU            |
//            |                     |Timer J fires
//            |                     |-
//            |                     |
//            |                     V
//            |               +-----------+
//            |               |           |
//            +-------------->| Terminated|
//                            |           |
//                            +-----------+

func (tx *serverTx) initNonInviteFSM() {
	// Define States
	tx.Fields().Debug("initialising non-INVITE transaction FSM")

	// Trying
	serverStateDefTrying := fsm.State{
		Index: serverStateTrying,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateTrying, fsm.NoAction},
			serverInputCancel:       {serverStateTrying, fsm.NoAction},
			serverInputUser1xx:      {serverStateProceeding, tx.actRespond},
			serverInputUser2xx:      {serverStateCompleted, tx.actFinal},
			serverInputUser300Plus:  {serverStateCompleted, tx.actFinal},
			serverInputTransportErr: {serverStateTerminated, tx.actTransErr},
		},
	}

	// Proceeding
	serverStateDefProceeding := fsm.State{
		Index: serverStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateProceeding, tx.actRespond},
			serverInputCancel:       {serverStateProceeding, fsm.NoAction},
			serverInputUser1xx:      {serverStateProceeding, tx.actRespond},
			serverInputUser2xx:      {serverStateCompleted, tx.actFinal},
			serverInputUser300Plus:  {serverStateCompleted, tx.actFinal},
			serverInputTransportErr: {serverStateTerminated, tx.actTransErr},
		},
	}

	// Completed
	serverStateDefCompleted := fsm.State{
		Index: serverStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateCompleted, tx.actRespond},
			serverInputCancel:       {serverStateCompleted, fsm.NoAction},
			serverInputUser1xx:      {serverStateCompleted, fsm.NoAction},
			serverInputUser2xx:      {serverStateCompleted, fsm.NoAction},
			serverInputUser300Plus:  {serverStateCompleted, fsm.NoAction},
			serverInputTimerJ:       {serverStateTerminated, tx.actDelete},
			serverInputTransportErr: {serverStateTerminated, tx.actTransErr},
		},
	}

	// Terminated
	serverStateDefTerminated := fsm.State{
		Index: serverStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:     {serverStateTerminated, fsm.NoAction},
			serverInputCancel:      {serverStateTerminated, fsm.NoAction},
			serverInputUser1xx:     {serverStateTerminated, fsm.NoAction},
			serverInputUser2xx:     {serverStateTerminated, fsm.NoAction},
			serverInputUser300Plus: {serverStateTerminated, fsm.NoAction},
			serverInputTimerJ:      {serverStateTerminated, fsm.NoAction},
			serverInputDelete:      {serverStateTerminated, tx.actDelete},
		},
	}

	// Define FSM
	fsm_, err := fsm.Define(
		serverStateDefTrying,
		serverStateDefProceeding,
		serverStateDefCompleted,
		serverStateDefTerminated,
	)
	if err != nil {
		tx.Fields().Error("define non-INVITE FSM failed: %s", err)

		return
	}

	tx.fsmMu.Lock()
	tx.fsm = fsm_
	tx.fsmMu.Unlock()
}

func (tx *serverTx) transportErr() {
	defer func() {
		if r := recover(); r != nil {
			log.Warn("sip transport error:%v", r)
		}
	}()

	var resStr string
	tx.mu.RLock()
	if tx.lastResp != nil {
		resStr = tx.lastResp.Short()
	}
	err := tx.lastErr
	tx.mu.RUnlock()

	err = &TxTransportError{
		fmt.Errorf("transaction failed to send %s: %w", resStr, err),
		tx.Key(),
		fmt.Sprintf("%p", tx),
	}

	select {
	case <-tx.done:
	case tx.errs <- err:
	}
}

func (tx *serverTx) timeoutErr() {
	defer func() {
		if r := recover(); r != nil {
			log.Warn("sip timeout error:%v", r)
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

func (tx *serverTx) delete() {
	select {
	case <-tx.done:
		return
	default:
	}
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("sip delete tx error", r)
		}
	}()

	tx.closeOnce.Do(func() {
		tx.mu.Lock()

		close(tx.done)
		close(tx.acks)
		close(tx.cancels)
		close(tx.errs)

		tx.mu.Unlock()

		tx.Fields().Debug("transaction done")
	})

	time.Sleep(time.Microsecond)

	tx.mu.Lock()
	if tx.timerI != nil {
		tx.timerI.Stop()
		tx.timerI = nil
	}
	if tx.timerG != nil {
		tx.timerG.Stop()
		tx.timerG = nil
	}
	if tx.timerH != nil {
		tx.timerH.Stop()
		tx.timerH = nil
	}
	if tx.timerJ != nil {
		tx.timerJ.Stop()
		tx.timerJ = nil
	}
	if tx.timer1xx != nil {
		tx.timer1xx.Stop()
		tx.timer1xx = nil
	}
	tx.mu.Unlock()
}

// Define actions.
// Send response
func (tx *serverTx) actRespond() fsm.Input {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp == nil {
		return fsm.NoInput
	}

	tx.Fields().Debug("act_respond")

	lastErr := tx.tpl.Send(lastResp)

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	return fsm.NoInput
}

func (tx *serverTx) actRespondComplete() fsm.Input {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp == nil {
		return fsm.NoInput
	}

	tx.Fields().Debug("act_respond_complete")

	lastErr := tx.tpl.Send(lastResp)

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	if !tx.reliable {
		tx.mu.Lock()
		if tx.timerG == nil {
			tx.Fields().Debug("timer_g set to %v", tx.timerGTime)

			tx.timerG = timing.AfterFunc(tx.timerGTime, func() {
				select {
				case <-tx.done:
					return
				default:
				}

				tx.Fields().Debug("timer_g fired")

				tx.fsmMu.RLock()
				if err := tx.fsm.Spin(serverInputTimerG); err != nil {
					tx.Fields().Error("spin FSM to server_input_timer_g failed: %s", err)
				}
				tx.fsmMu.RUnlock()
			})
		} else {
			tx.timerGTime *= 2
			if tx.timerGTime > T2 {
				tx.timerGTime = T2
			}

			tx.Fields().Debug("timer_g reset to %v", tx.timerGTime)

			tx.timerG.Reset(tx.timerGTime)
		}
		tx.mu.Unlock()
	}

	tx.mu.Lock()
	if tx.timerH == nil {
		tx.Fields().Debug("timer_h set to %v", TimerH)

		tx.timerH = timing.AfterFunc(TimerH, func() {
			select {
			case <-tx.done:
				return
			default:
			}

			tx.Fields().Debug("timer_h fired")

			tx.fsmMu.RLock()
			if err := tx.fsm.Spin(serverInputTimerH); err != nil {
				tx.Fields().Error("spin FSM to server_input_timer_h failed: %s", err)
			}
			tx.fsmMu.RUnlock()
		})
	}
	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *serverTx) actRespondAccept() fsm.Input {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp == nil {
		return fsm.NoInput
	}

	tx.Fields().Debug("act_respond_accept")

	lastErr := tx.tpl.Send(lastResp)

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	tx.mu.Lock()

	tx.timerL = timing.AfterFunc(TimerL, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_l fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(serverInputTimerL); err != nil {
			tx.Fields().Error("spin FSM to server_input_timer_l failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})
	tx.mu.Unlock()

	return fsm.NoInput
}

func (tx *serverTx) actPassUpAck() fsm.Input {
	tx.Fields().Debug("act_pass_up_ack")

	tx.mu.RLock()
	ack := tx.lastAck
	tx.mu.RUnlock()

	if ack != nil {
		select {
		case <-tx.done:
		case tx.acks <- ack:
		}
	}

	return fsm.NoInput
}

// Send final response
func (tx *serverTx) actFinal() fsm.Input {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp == nil {
		return fsm.NoInput
	}

	tx.Fields().Debug("act_final")

	lastErr := tx.tpl.Send(tx.lastResp)

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	tx.mu.Lock()

	tx.Fields().Debug("timer_j set to %v", TimerJ)

	tx.timerJ = timing.AfterFunc(TimerJ, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_j fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(serverInputTimerJ); err != nil {
			tx.Fields().Error("spin FSM to server_input_timer_j failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})

	tx.mu.Unlock()

	return fsm.NoInput
}

// Inform user of transport error
func (tx *serverTx) actTransErr() fsm.Input {
	tx.Fields().Debug("act_trans_err")

	tx.transportErr()

	return serverInputDelete
}

// Inform user of timeout error
func (tx *serverTx) actTimeout() fsm.Input {
	tx.Fields().Debug("act_timeout")

	tx.timeoutErr()

	return serverInputDelete
}

// Just delete the transaction.
func (tx *serverTx) actDelete() fsm.Input {
	tx.Fields().Debug("act_delete")

	tx.delete()

	return fsm.NoInput
}

// Send response and delete the transaction.
func (tx *serverTx) actRespondDelete() fsm.Input {
	tx.Fields().Debug("act_respond_delete")

	tx.delete()

	tx.mu.RLock()
	lastErr := tx.tpl.Send(tx.lastResp)
	tx.mu.RUnlock()

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	return fsm.NoInput
}

func (tx *serverTx) actConfirm() fsm.Input {
	tx.Fields().Debug("act_confirm")

	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("sip act confirm error", r)
		}
	}()

	tx.mu.Lock()

	if tx.timerG != nil {
		tx.timerG.Stop()
		tx.timerG = nil
	}

	if tx.timerH != nil {
		tx.timerH.Stop()
		tx.timerH = nil
	}

	tx.Fields().Debug("timer_i set to %v", TimerI)

	tx.timerI = timing.AfterFunc(TimerI, func() {
		select {
		case <-tx.done:
			return
		default:
		}

		tx.Fields().Debug("timer_i fired")

		tx.fsmMu.RLock()
		if err := tx.fsm.Spin(serverInputTimerI); err != nil {
			tx.Fields().Error("spin FSM to server_input_timer_i failed: %s", err)
		}
		tx.fsmMu.RUnlock()
	})

	tx.mu.Unlock()

	tx.mu.RLock()
	ack := tx.lastAck
	tx.mu.RUnlock()

	if ack != nil {
		select {
		case <-tx.done:
		case tx.acks <- ack:
		}
	}

	return fsm.NoInput
}

func (tx *serverTx) actCancel() fsm.Input {
	tx.Fields().Debug("act_cancel")

	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("sip act cancel error", r)
		}
	}()

	tx.mu.RLock()
	cancel := tx.lastCancel
	tx.mu.RUnlock()

	if cancel != nil {
		select {
		case <-tx.done:
		case tx.cancels <- cancel:
		}
	}

	return fsm.NoInput
}
