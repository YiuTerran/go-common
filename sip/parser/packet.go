package parser

import (
	"bytes"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/sip"
	"strings"
)

/**
  *  @author tryao
  *  @date 2023/01/12 17:59
**/

// PacketParser 直接解析
type PacketParser struct {
	headerParsers map[string]HeaderParser
	fields        log.Fields
}

func NewPacketParser(fields log.Fields) *PacketParser {
	p := &PacketParser{}
	p.fields = log.Fields{
		"parser_ptr": fmt.Sprintf("%p", p),
	}.WithFields(fields).WithPrefix("parser.Parser")
	p.headerParsers = make(map[string]HeaderParser)
	for headerName, headerParser := range defaultHeaderParsers() {
		p.SetHeaderParser(headerName, headerParser)
	}
	return p
}

func (pp *PacketParser) Write(p []byte) (n int, err error) {
	panic("should not use")
}

func (pp *PacketParser) SetHeaderParser(headerName string, headerParser HeaderParser) {
	headerName = strings.ToLower(headerName)
	pp.headerParsers[headerName] = headerParser
}

func (pp *PacketParser) Stop() {
}

func (pp *PacketParser) String() string {
	if pp == nil {
		return "Parser <nil>"
	}
	return fmt.Sprintf("Parser %p", pp)
}

func (pp *PacketParser) ParseHeader(headerText string) (headers []sip.Header, err error) {
	headers = make([]sip.Header, 0)

	colonIdx := strings.Index(headerText, ":")
	if colonIdx == -1 {
		err = fmt.Errorf("field name with no value in header: %s", headerText)
		return
	}

	fieldName := strings.TrimSpace(headerText[:colonIdx])
	lowerFieldName := strings.ToLower(fieldName)
	fieldText := strings.TrimSpace(headerText[colonIdx+1:])
	if headerParser, ok := pp.headerParsers[lowerFieldName]; ok {
		// We have a registered StreamParser for this header type - use it.
		headers, err = headerParser(lowerFieldName, fieldText)
	} else {
		// We have no registered StreamParser for this header type,
		// so we encapsulate the header data in a GenericHeader struct.
		pp.Fields().Debug("no StreamParser for header type %s", fieldName)

		header := sip.GenericHeader{
			HeaderName: fieldName,
			Contents:   fieldText,
		}
		headers = []sip.Header{&header}
	}

	return
}

func (pp *PacketParser) Fields() log.Fields {
	return pp.fields
}

func (pp *PacketParser) parseStartLine(startLine string) (msg sip.Message, err error) {
	if isRequest(startLine) {
		method, recipient, sipVersion, err := ParseRequestLine(startLine)
		if err == nil {
			msg = sip.NewRequest("", method, recipient, sipVersion, []sip.Header{}, "", nil)
		} else {
			return nil, err
		}
	} else if isResponse(startLine) {
		sipVersion, statusCode, reason, err := ParseStatusLine(startLine)
		if err == nil {
			msg = sip.NewResponse("", sipVersion, statusCode, reason, []sip.Header{}, "", nil)
		} else {
			return nil, err
		}
	} else {
		return nil, InvalidStartLineError(
			fmt.Sprintf("transmission beginning '%s' is not a SIP message", startLine))
	}
	return
}

func (pp *PacketParser) fillHeaders(msg sip.Message, lines []string) {
	//分析header
	var buffer bytes.Buffer
	headers := make([]sip.Header, 0)
	flushBuffer := func() {
		if buffer.Len() > 0 {
			newHeaders, err := pp.ParseHeader(buffer.String())
			if err == nil {
				headers = append(headers, newHeaders...)
			} else {
				pp.Fields().Warn("skip header '%s' due to error: %s", buffer, err)
			}
			buffer.Reset()
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.Contains(abnfWs, string(line[0])) {
			// This line starts a new header.
			// Parse anything currently in the buffer, then store the new header line in the buffer.
			flushBuffer()
			buffer.WriteString(line)
		} else if buffer.Len() > 0 {
			// This is a continuation line, so just add it to the buffer.
			buffer.WriteString(" ")
			buffer.WriteString(line)
		} else {
			// This is a continuation line, but also the first line of the whole header section.
			// Discard it and log.
			pp.Fields().Debug(
				"discard unexpected continuation line '%s' at start of header block in message '%s'",
				line,
				msg.Short(),
			)
		}
	}
	flushBuffer()
	// Store the headers in the message object.
	for _, header := range headers {
		msg.AppendHeader(header)
	}
}

func (pp *PacketParser) fillBody(msg sip.Message, body string, bodyLen int) error {
	// RFC 3261 - 18.3.
	if len(body) != bodyLen {
		return &sip.BrokenMessageError{
			Err: fmt.Errorf("incomplete message body: read %d bytes, expected %d bytes", len(body), bodyLen),
			Msg: msg.String(),
		}
	}
	if strings.TrimSpace(body) != "" {
		msg.SetBody(body, false)
	}
	return nil
}

func (pp *PacketParser) ParseMessage(data []byte) (sip.Message, error) {
	bodyLen := getBodyLength(data)
	if bodyLen == -1 {
		return nil, InvalidMessageFormat("format error")
	}
	bodyStart := len(data) - bodyLen
	parts := strings.Split(string(data[:bodyStart]), "\r\n")
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) < 1 {
		return nil, InvalidMessageFormat("format error")
	}
	//分析startLine
	msg, err := pp.parseStartLine(filtered[0])
	if err != nil {
		return nil, err
	}
	pp.fillHeaders(msg, filtered[1:])
	if err = pp.fillBody(msg, string(data[bodyStart:]), bodyLen); err != nil {
		return nil, err
	}
	return msg, nil
}
