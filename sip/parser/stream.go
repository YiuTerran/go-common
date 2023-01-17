package parser

// Forked from github.com/StefanKopieczek/gossip by @StefanKopieczek

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/sip"
	"go.uber.org/atomic"
	"sync"
)

type StreamParser struct {
	*PacketParser
	input *parserBuffer

	output chan<- sip.Message
	errs   chan<- error

	stopped atomic.Bool
	done    chan struct{}

	mu sync.Mutex
}

// NewStreamParser create a new Parser.
//
// Parsed SIP messages will be sent down the 'output' chan provided.
// Any errors which force the StreamParser to terminate will be sent down the 'errs' chan provided.
//
// If streamed=false, each Write call to the StreamParser should contain data for one complete SIP message.
// If streamed=true, Write calls can contain a portion of a full SIP message.
// The end of one message and the start of the next may be provided in a single call to Write.
// When streamed=true, all SIP messages provided must have a Content-Length header.
// SIP messages without a Content-Length will cause the StreamParser to permanently stop, and will result in an error on the errs chan.
// 'streamed' should be set to true whenever the caller cannot reliably identify the starts and ends of messages from the transport frames,
// e.g. when using streamed protocols such as TCP.
func NewStreamParser(
	output chan<- sip.Message,
	errs chan<- error,
	fields log.Fields,
) Parser {
	p := &StreamParser{
		done: make(chan struct{}),
	}
	p.PacketParser = NewPacketParser(fields)
	p.output = output
	p.errs = errs
	// Create a managed buffer to allow message data to be asynchronously provided to the StreamParser, and
	// to allow the StreamParser to block until enough data is available to parse.
	p.input = newParserBuffer(p.Fields())
	// Done for input a line at a time, and produce SipMessages to send down p.output.
	go p.parse(p.done)
	return p
}

// Write for streamed only
func (p *StreamParser) Write(data []byte) (int, error) {
	if p.stopped.Load() {
		return 0, WriteError(fmt.Sprintf("cannot write data to stopped %s", p))
	}

	var (
		num int
		err error
	)

	num, err = p.input.Write(data)
	if err != nil {
		err = WriteError(fmt.Sprintf("%s write data failed: %s", p, err))
		return num, err
	}
	return num, nil
}

// Stop StreamParser processing, and allow all resources to be garbage collected.
// The StreamParser will not release its resources until Stop() is called,
// even if the StreamParser object itself is garbage collected.
func (p *StreamParser) Stop() {
	if !p.stopped.CompareAndSwap(false, true) {
		return
	}
	p.input.Stop()
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	<-done
}

// Consume input lines one at a time, producing sip.Message objects and sending them down p.output.
func (p *StreamParser) parse(done chan<- struct{}) {
	defer close(done)

	var msg sip.Message
	var skipStreamedErr bool

	for {
		// Parse the StartLine.
		startLine, err := p.input.NextLine()
		if err != nil {
			break
		}
		var termErr error
		if startLine == "" {
			continue
		} else {
			msg, termErr = p.parseStartLine(startLine)
		}
		if termErr != nil {
			termErr = InvalidStartLineError(fmt.Sprintf("%s failed to parse first line of message: %s", p, termErr))
			if !skipStreamedErr {
				skipStreamedErr = true
				p.errs <- termErr
			}
			continue
		} else {
			skipStreamedErr = false
		}
		lines := make([]string, 0)
		for {
			line, err := p.input.NextLine()
			if err != nil || len(line) == 0 {
				break
			}
			lines = append(lines, line)
		}
		p.fillHeaders(msg, lines)
		// Determine the length of the body, so we know when to stop parsing this message.
		// Use the content-length header to identify the end of the message.
		contentLengthHeaders := msg.GetHeaders("Content-Length")
		if len(contentLengthHeaders) == 0 {
			skipStreamedErr = true

			termErr := &sip.MalformedMessageError{
				Err: fmt.Errorf("missing required 'Content-Length' header"),
				Msg: msg.String(),
			}
			p.errs <- termErr

			continue
		} else if len(contentLengthHeaders) > 1 {
			skipStreamedErr = true

			var errbuf bytes.Buffer
			errbuf.WriteString("multiple 'Content-Length' headers on message '")
			errbuf.WriteString(msg.Short())
			errbuf.WriteString(fmt.Sprintf("'; StreamParser: %s:\n", p))
			for _, header := range contentLengthHeaders {
				errbuf.WriteString("\t")
				errbuf.WriteString(header.String())
			}
			termErr := &sip.MalformedMessageError{
				Err: errors.New(errbuf.String()),
				Msg: msg.String(),
			}
			p.errs <- termErr
			continue
		}
		contentLength := int(*(contentLengthHeaders[0].(*sip.ContentLength)))
		body, err := p.input.NextChunk(contentLength)
		if err != nil {
			termErr := &sip.BrokenMessageError{
				Err: fmt.Errorf("read message body failed: %w", err),
				Msg: msg.String(),
			}
			p.errs <- termErr
			continue
		}
		if err = p.fillBody(msg, body, contentLength); err != nil {
			continue
		}
		p.output <- msg
	}
	return
}
