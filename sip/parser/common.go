package parser

import (
	"bytes"
	"fmt"
	"github.com/samber/lo"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/sip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

/**
  *  @author tryao
  *  @date 2023/01/12 17:59
**/

// The whitespace characters recognised by the Augmented Backus-Naur Form syntax
// that SIP uses (RFC 3261 S.25).
const abnfWs = " \t"

// The maximum permissible CSeq number in a SIP message (2**31 - 1).
// C.f. RFC 3261 S. 8.1.1.5.
const maxCseq = 2147483647

// The buffer size of the StreamParser input channel.

// Parser converts the raw bytes of SIP messages into sip.Message objects.
// It allows
type Parser interface {
	// Write implements io.Writer. Queues the given bytes to be parsed.
	// If the StreamParser has terminated due to a previous fatal error, it will return n=0 and an appropriate error.
	// Otherwise, it will return n=len(p) and err=nil.
	// Note that err=nil does not indicate that the data provided is valid - simply that the data was successfully queued for parsing.
	Write(p []byte) (n int, err error)
	// SetHeaderParser register a custom header StreamParser for a particular header type.
	// This will overwrite any existing registered StreamParser for that header type.
	// If a StreamParser is not available for a header type in a message, the StreamParser will produce a sip.GenericHeader struct.
	SetHeaderParser(headerName string, headerParser HeaderParser)
	Stop()
	String() string
	ParseHeader(headerText string) (headers []sip.Header, err error)
}

// A HeaderParser is any function that turns raw header data into one or more Header objects.
// The HeaderParser will receive arguments of the form ("max-forwards", "70").
// It should return a slice of headers, which should have length > 1 unless it also returns an error.
type HeaderParser func(headerName string, headerData string) ([]sip.Header, error)

func defaultHeaderParsers() map[string]HeaderParser {
	return map[string]HeaderParser{
		"to":             parseAddressHeader,
		"t":              parseAddressHeader,
		"from":           parseAddressHeader,
		"f":              parseAddressHeader,
		"contact":        parseAddressHeader,
		"m":              parseAddressHeader,
		"call-id":        parseCallId,
		"i":              parseCallId,
		"cseq":           parseCSeq,
		"via":            parseViaHeader,
		"v":              parseViaHeader,
		"max-forwards":   parseMaxForwards,
		"content-length": parseContentLength,
		"l":              parseContentLength,
		"expires":        parseExpires,
		"user-agent":     parseUserAgent,
		"allow":          parseAllow,
		"content-type":   parseContentType,
		"c":              parseContentType,
		"accept":         parseAccept,
		"require":        parseRequire,
		"supported":      parseSupported,
		"k":              parseSupported,
		"route":          parseRouteHeader,
		"record-route":   parseRecordRouteHeader,
		//"content-encoding","e"
		//"subject":          "s",
	}
}

// ParseMessage
// parse a SIP message by creating a StreamParser on the fly.
// This is more costly than reusing a StreamParser, but is necessary when we do not
// have a guarantee that all messages coming over a connection are from the
// same endpoint (e.g. UDP).
func ParseMessage(msgData []byte, fields log.Fields) (sip.Message, error) {
	parser := NewPacketParser(fields)
	return parser.ParseMessage(msgData)
}

// Calculate the size of a SIP message's body, given the entire contents of the message as a byte array.
func getBodyLength(data []byte) int {
	s := string(data)

	// Body starts with first character following a double-CRLF.
	idx := strings.Index(s, "\r\n\r\n")
	if idx == -1 {
		return -1
	}

	bodyStart := idx + 4

	return len(s) - bodyStart
}

// Heuristic to determine if the given transmission looks like a SIP request.
// It is guaranteed that any RFC3261-compliant request will pass this test,
// but invalid messages may not necessarily be rejected.
func isRequest(startLine string) bool {
	// SIP request lines contain precisely two spaces.
	if strings.Count(startLine, " ") != 2 {
		return false
	}

	// Check that the version string starts with SIP.
	parts := strings.Split(startLine, " ")
	if len(parts) < 3 {
		return false
	} else if len(parts[2]) < 3 {
		return false
	} else {
		return strings.ToUpper(parts[2][:3]) == "SIP"
	}
}

// Heuristic to determine if the given transmission looks like a SIP response.
// It is guaranteed that any RFC3261-compliant response will pass this test,
// but invalid messages may not necessarily be rejected.
func isResponse(startLine string) bool {
	// SIP status lines contain at least two spaces.
	if strings.Count(startLine, " ") < 2 {
		return false
	}

	// Check that the version string starts with SIP.
	parts := strings.Split(startLine, " ")
	if len(parts) < 3 {
		return false
	} else if len(parts[0]) < 3 {
		return false
	} else {
		return strings.ToUpper(parts[0][:3]) == "SIP"
	}
}

// ParseRequestLine parse the first line of a SIP request, e.g:
//
//	INVITE bob@example.com SIP/2.0
//	REGISTER jane@telco.com SIP/1.0
func ParseRequestLine(requestLine string) (
	method sip.RequestMethod, recipient sip.Uri, sipVersion string, err error) {
	parts := strings.Split(requestLine, " ")
	if len(parts) != 3 {
		err = fmt.Errorf("request line should have 2 spaces: '%s'", requestLine)
		return
	}

	method = sip.RequestMethod(strings.ToUpper(parts[0]))
	recipient, err = ParseUri(parts[1])
	sipVersion = parts[2]

	switch recipient.(type) {
	case *sip.WildcardUri:
		err = fmt.Errorf("wildcard URI '*' not permitted in request line: '%s'", requestLine)
	}

	return
}

// ParseStatusLine parse the first line of a SIP response, e.g:
//
//	SIP/2.0 200 OK
//	SIP/1.0 403 Forbidden
func ParseStatusLine(statusLine string) (
	sipVersion string, statusCode sip.StatusCode, reasonPhrase string, err error) {
	parts := strings.Split(statusLine, " ")
	if len(parts) < 3 {
		err = fmt.Errorf("status line has too few spaces: '%s'", statusLine)
		return
	}

	sipVersion = parts[0]
	statusCodeRaw, err := strconv.ParseUint(parts[1], 10, 16)
	statusCode = sip.StatusCode(statusCodeRaw)
	reasonPhrase = strings.Join(parts[2:], " ")

	return
}

// ParseUri converts a string representation of a URI into a Uri object.
// If the URI is malformed, or the URI schema is not recognised, an error is returned.
// URIs have the general form of schema:address.
func ParseUri(uriStr string) (uri sip.Uri, err error) {
	if strings.TrimSpace(uriStr) == "*" {
		// Wildcard '*' URI used in the Contact headers of REGISTERs when unregistering.
		return sip.WildcardUri{}, nil
	}

	colonIdx := strings.Index(uriStr, ":")
	if colonIdx == -1 {
		err = fmt.Errorf("no ':' in URI %s", uriStr)
		return
	}

	switch strings.ToLower(uriStr[:colonIdx]) {
	case "sip":
		var sipUri sip.SipUri
		sipUri, err = ParseSipUri(uriStr)
		uri = &sipUri
	case "sips":
		// SIPS URIs have the same form as SIP uris, so we use the same StreamParser.
		var sipUri sip.SipUri
		sipUri, err = ParseSipUri(uriStr)
		uri = &sipUri
	default:
		err = fmt.Errorf("unsupported URI schema %s", uriStr[:colonIdx])
	}

	return
}

// ParseSipUri converts a string representation of a SIP or SIPS URI into a SipUri object.
// sip:user:password@host:port;uri-parameters?headers
func ParseSipUri(uriStr string) (uri sip.SipUri, err error) {
	//direct parse, get query param
	var r1 *url.URL
	r1, err = url.Parse(uriStr)
	if err != nil {
		return
	}
	if r1.Scheme == "sips" {
		uri.FIsEncrypted = true
	} else if r1.Scheme != "sip" {
		err = fmt.Errorf("wrong schema:%s", r1.Scheme)
		return
	}
	if len(r1.RawQuery) > 0 {
		//和http不同，uri-header里面不允许有单独的key
		qs := strings.Split(r1.RawQuery, "&")
		for _, q := range qs {
			if !strings.Contains(q, "=") {
				err = &sip.MalformedMessageError{
					Err: fmt.Errorf("query string without `=` is not allowed"),
					Msg: uriStr,
				}
				return
			}
		}
	}
	//parse userInfo, host and uri-params
	//replace ";" to "?" and "&" like query params
	var sb strings.Builder
	sb.WriteString("//")
	hostIndex := strings.Index(r1.Opaque, "@")
	if hostIndex < 0 {
		hostIndex = strings.Index(r1.Opaque, ";")
	}
	firstParam := true
	for i := 0; i < len(r1.Opaque); i++ {
		ch := r1.Opaque[i]
		if ch == ';' && i >= hostIndex {
			if firstParam {
				ch = '?'
				firstParam = false
			} else {
				ch = '&'
			}
		}
		sb.WriteByte(ch)
	}
	r2, err := url.Parse(sb.String())
	if err != nil {
		return
	}
	uri.FHost = strings.ToLower(r2.Hostname())
	if r2.Port() != "" {
		portRaw64, _ := strconv.ParseUint(r2.Port(), 10, 16)
		uri.FPort = lo.ToPtr(sip.Port(portRaw64))
	}
	if r2.User != nil {
		uri.FUser = sip.String{Str: r2.User.Username()}
		if pass, ok := r2.User.Password(); ok {
			uri.FPassword = sip.String{Str: pass}
		}
	}

	uri.FHeaders = sip.NewParams(sip.UriHeaders)
	uri.FUriParams = sip.NewParams(sip.UriParams)
	for param, vs := range map[sip.Params]url.Values{
		uri.FHeaders:   r1.Query(),
		uri.FUriParams: r2.Query()} {
		for k, v := range vs {
			if len(v) == 0 {
				param.Add(k, sip.String{Str: ""})
				continue
			}
			for _, s := range v {
				param.Add(strings.ToLower(k), sip.String{Str: strings.ToLower(s)})
			}
		}
	}
	return
}

// ParseHostPort parse a text representation of a host[:port] pair.
// The port may or may not be present, so we represent it with a *uint16,
// and return 'nil' if no port was present.
func ParseHostPort(rawText string) (host string, port *sip.Port, err error) {
	colonIdx := strings.Index(rawText, ":")
	if colonIdx == -1 {
		host = rawText
		return
	}

	// Surely there must be a better way..!
	var portRaw64 uint64
	var portRaw16 uint16
	host = rawText[:colonIdx]
	portRaw64, err = strconv.ParseUint(rawText[colonIdx+1:], 10, 16)
	portRaw16 = uint16(portRaw64)
	port = (*sip.Port)(&portRaw16)

	return
}

// ParseHeaderParams is general utility method for parsing 'key=value' parameters.
// Takes a string (source), ensures that it begins with the 'start' character provided,
// and then parses successive key/value pairs separated with 'sep',
// until either 'end' is reached or there are no characters remaining.
// A map of key to value will be returned, along with the number of characters consumed.
// Provide 0 for start or end to indicate that there is no starting/ending delimiter.
// If quoteValues is true, values can be enclosed in double-quotes which will be validated by the
// StreamParser and omitted from the returned map.
// If permitSingletons is true, keys with no values are permitted.
// These will result in a nil value in the returned map.
func ParseHeaderParams(
	source string,
	start uint8,
	sep uint8,
	end uint8,
	quoteValues bool,
	permitSingletons bool,
) (
	params sip.Params, consumed int, err error) {

	params = sip.NewParams(sip.HeaderParams)

	if len(source) == 0 {
		// Key-value section is completely empty; return defaults.
		return
	}
	source, _ = url.PathUnescape(source)
	// Ensure the starting character is correct.
	if start != 0 {
		if source[0] != start {
			err = fmt.Errorf(
				"expected %c at start of key-value section; got %c. section was %s",
				start,
				source[0],
				source,
			)
			return
		}
		consumed++
	}

	// Stateful parse the given string one character at a time.
	var buffer bytes.Buffer
	var key string
	parsingKey := true // false implies we are parsing a value
	inQuotes := false
parseLoop:
	for ; consumed < len(source); consumed++ {
		switch source[consumed] {
		case end:
			if inQuotes {
				// We read an end character, but since we're inside quotations we should
				// treat it as a literal part of the value.
				buffer.WriteString(string(end))
				continue
			}

			break parseLoop

		case sep:
			if inQuotes {
				// We read a separator character, but since we're inside quotations
				// we should treat it as a literal part of the value.
				buffer.WriteString(string(sep))
				continue
			}
			if parsingKey && permitSingletons {
				params.Add(buffer.String(), nil)
			} else if parsingKey {
				err = fmt.Errorf(
					"singleton param '%s' when parsing params which disallow singletons: \"%s\"",
					buffer.String(),
					source,
				)
				return
			} else {
				params.Add(key, sip.String{Str: buffer.String()})
			}
			buffer.Reset()
			parsingKey = true

		case '"':
			if !quoteValues {
				// We hit a quote character, but since quoting is turned off we treat it as a literal.
				buffer.WriteString("\"")
				continue
			}

			if parsingKey {
				// Quotes are never allowed in keys.
				err = fmt.Errorf("unexpected '\"' in parameter key in params \"%s\"", source)
				return
			}

			if !inQuotes && buffer.Len() != 0 {
				// We hit an initial quote midway through a value; that's not allowed.
				err = fmt.Errorf("unexpected '\"' in params \"%s\"", source)
				return
			}

			if inQuotes &&
				consumed != len(source)-1 &&
				source[consumed+1] != sep {
				// We hit an end-quote midway through a value; that's not allowed.
				err = fmt.Errorf("unexpected character %c after quoted param in \"%s\"",
					source[consumed+1], source)

				return
			}

			inQuotes = !inQuotes

		case '=':
			if buffer.Len() == 0 {
				err = fmt.Errorf("key of length 0 in params \"%s\"", source)
				return
			}
			if !parsingKey {
				err = fmt.Errorf("unexpected '=' char in value token: \"%s\"", source)
				return
			}
			key = buffer.String()
			buffer.Reset()
			parsingKey = false

		default:
			if !inQuotes && strings.Contains(abnfWs, string(source[consumed])) {
				// Skip unquoted whitespace.
				continue
			}

			buffer.WriteString(string(source[consumed]))
		}
	}

	// The param string has ended. Check that it ended in a valid place, and then store off the
	// contents of the buffer.
	if inQuotes {
		err = fmt.Errorf("unclosed quotes in parameter string: %s", source)
	} else if parsingKey && permitSingletons {
		params.Add(buffer.String(), nil)
	} else if parsingKey {
		err = fmt.Errorf("singleton param '%s' when parsing params which disallow singletons: \"%s\"",
			buffer.String(), source)
	} else {
		params.Add(key, sip.String{Str: buffer.String()})
	}
	return
}

// Parse a `To`, `From` or `Contact` header line, producing one or more logical SipHeaders.
func parseAddressHeader(headerName string, headerText string) (
	headers []sip.Header, err error) {
	switch headerName {
	case "to", "from", "contact", "t", "f", "m":
		var displayNames []sip.MaybeString
		var uris []sip.Uri
		var paramSets []sip.Params

		// Perform the actual parsing. The rest of this method is just typeclass bookkeeping.
		displayNames, uris, paramSets, err = ParseAddressValues(headerText)

		if err != nil {
			return
		}
		if len(displayNames) != len(uris) || len(uris) != len(paramSets) {
			// This shouldn't happen unless ParseAddressValues is bugged.
			err = fmt.Errorf("internal StreamParser error: parsed param mismatch. "+
				"%d display names, %d uris and %d param sets "+
				"in %s",
				len(displayNames), len(uris), len(paramSets),
				headerText)
			return
		}

		// Build a slice of headers of the appropriate kind, populating them with the values parsed above.
		// It is assumed that all headers returned by ParseAddressValues are of the same kind,
		// although we do not check for this below.
		for idx := 0; idx < len(displayNames); idx++ {
			var header sip.Header
			if headerName == "to" || headerName == "t" {
				if idx > 0 {
					// Only a single To header is permitted in a SIP message.
					return nil,
						fmt.Errorf("multiple to: headers in message:\n%s: %s",
							headerName, headerText)
				}
				switch uris[idx].(type) {
				case sip.WildcardUri:
					// The Wildcard '*' URI is only permitted in Contact headers.
					err = fmt.Errorf(
						"wildcard uri not permitted in to: header: %s",
						headerText,
					)
					return
				default:
					toHeader := sip.ToHeader{
						DisplayName: displayNames[idx],
						Address:     uris[idx],
						Params:      paramSets[idx],
					}
					header = &toHeader
				}
			} else if headerName == "from" || headerName == "f" {
				if idx > 0 {
					// Only a single From header is permitted in a SIP message.
					return nil,
						fmt.Errorf(
							"multiple from: headers in message:\n%s: %s",
							headerName,
							headerText,
						)
				}
				switch uris[idx].(type) {
				case sip.WildcardUri:
					// The Wildcard '*' URI is only permitted in Contact headers.
					err = fmt.Errorf(
						"wildcard uri not permitted in from: header: %s",
						headerText,
					)
					return
				default:
					fromHeader := sip.FromHeader{
						DisplayName: displayNames[idx],
						Address:     uris[idx],
						Params:      paramSets[idx],
					}
					header = &fromHeader
				}
			} else if headerName == "contact" || headerName == "m" {
				switch uris[idx].(type) {
				case sip.ContactUri:
					if uris[idx].(sip.ContactUri).IsWildcard() {
						if paramSets[idx].Length() > 0 {
							// Wildcard headers do not contain parameters.
							err = fmt.Errorf(
								"wildcard contact header should contain no parameters: '%s",
								headerText,
							)
							return
						}
						if displayNames[idx] != nil {
							// Wildcard headers do not contain display names.
							err = fmt.Errorf(
								"wildcard contact header should contain no display name %s",
								headerText,
							)
							return
						}
					}
					contactHeader := sip.ContactHeader{
						DisplayName: displayNames[idx],
						Address:     uris[idx].(sip.ContactUri),
						Params:      paramSets[idx],
					}
					header = &contactHeader
				default:
					// URIs in contact headers are restricted to being either SIP URIs or 'Contact: *'.
					return nil,
						fmt.Errorf(
							"uri %s not valid in Contact header. Must be SIP uri or '*'",
							uris[idx].String(),
						)
				}
			}

			headers = append(headers, header)
		}
	}

	return
}

// Parse a string representation of a CSeq header, returning a slice of at most one CSeq.
func parseCSeq(headerName string, headerText string) (
	headers []sip.Header, err error) {
	var seq sip.CSeq

	parts := SplitByWhitespace(headerText)
	if len(parts) != 2 {
		err = fmt.Errorf(
			"CSeq field should have precisely one whitespace section: '%s'",
			headerText,
		)
		return
	}

	var seqno uint64
	seqno, err = strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return
	}

	if seqno > maxCseq {
		err = fmt.Errorf("invalid CSeq %d: exceeds maximum permitted value "+
			"2**31 - 1", seqno)
		return
	}

	seq.SeqNo = uint32(seqno)
	seq.MethodName = sip.RequestMethod(strings.TrimSpace(parts[1]))

	if strings.Contains(string(seq.MethodName), ";") {
		err = fmt.Errorf("unexpected ';' in CSeq body: %s", headerText)
		return
	}

	headers = []sip.Header{&seq}

	return
}

// Parse a string representation of a Call-ID header, returning a slice of at most one CallID.
func parseCallId(headerName string, headerText string) (
	headers []sip.Header, err error) {
	headerText = strings.TrimSpace(headerText)
	var callId = sip.CallID(headerText)

	if strings.ContainsAny(string(callId), abnfWs) {
		err = fmt.Errorf("unexpected whitespace in CallID header body '%s'", headerText)
		return
	}
	if strings.Contains(string(callId), ";") {
		err = fmt.Errorf("unexpected semicolon in CallID header body '%s'", headerText)
		return
	}
	if len(string(callId)) == 0 {
		err = fmt.Errorf("empty Call-ID body")
		return
	}

	headers = []sip.Header{&callId}

	return
}

// Parse a string representation of a Via header, returning a slice of at most one ViaHeader.
// Note that although Via headers may contain a comma-separated list, RFC 3261 makes it clear that
// these should not be treated as separate logical Via headers, but as multiple values on a single
// Via header.
func parseViaHeader(headerName string, headerText string) (
	headers []sip.Header, err error) {
	sections := strings.Split(headerText, ",")
	var via = sip.ViaHeader{}
	for _, section := range sections {
		var hop sip.ViaHop
		parts := strings.Split(section, "/")

		if len(parts) < 3 {
			err = fmt.Errorf("not enough protocol parts in via header: '%s'", parts)
			return
		}

		parts[2] = strings.Join(parts[2:], "/")

		// The transport part ends when whitespace is reached, but may also start with
		// whitespace.
		// So the end of the transport part is the first whitespace char following the
		// first non-whitespace char.
		initialSpaces := len(parts[2]) - len(strings.TrimLeft(parts[2], abnfWs))
		sentByIdx := strings.IndexAny(parts[2][initialSpaces:], abnfWs) + initialSpaces + 1
		if sentByIdx == 0 {
			err = fmt.Errorf("expected whitespace after sent-protocol part "+
				"in via header '%s'", section)
			return
		} else if sentByIdx == 1 {
			err = fmt.Errorf("empty transport field in via header '%s'", section)
			return
		}

		hop.ProtocolName = strings.TrimSpace(parts[0])
		hop.ProtocolVersion = strings.TrimSpace(parts[1])
		hop.Transport = strings.TrimSpace(parts[2][:sentByIdx-1])

		if len(hop.ProtocolName) == 0 {
			err = fmt.Errorf("no protocol name provided in via header '%s'", section)
		} else if len(hop.ProtocolVersion) == 0 {
			err = fmt.Errorf("no version provided in via header '%s'", section)
		} else if len(hop.Transport) == 0 {
			err = fmt.Errorf("no transport provided in via header '%s'", section)
		}
		if err != nil {
			return
		}

		viaBody := parts[2][sentByIdx:]

		paramsIdx := strings.Index(viaBody, ";")
		var host string
		var port *sip.Port
		if paramsIdx == -1 {
			// There are no header parameters, so the rest of the Via body is part of the host[:post].
			host, port, err = ParseHostPort(viaBody)
			hop.Host = host
			hop.Port = port
			if err != nil {
				return
			}
			hop.Params = sip.NewParams(sip.HeaderParams)
		} else {
			host, port, err = ParseHostPort(viaBody[:paramsIdx])
			if err != nil {
				return
			}
			hop.Host = host
			hop.Port = port

			hop.Params, _, err = ParseHeaderParams(viaBody[paramsIdx:],
				';', ';', 0, true, true)
		}
		via = append(via, &hop)
	}

	headers = []sip.Header{via}
	return
}

// Parse a string representation of a Max-Forwards header into a slice of at most one MaxForwards header object.
func parseMaxForwards(headerName string, headerText string) (
	headers []sip.Header, err error) {
	var maxForwards sip.MaxForwards
	var value uint64
	value, err = strconv.ParseUint(strings.TrimSpace(headerText), 10, 32)
	maxForwards = sip.MaxForwards(value)

	headers = []sip.Header{&maxForwards}
	return
}

func parseExpires(headerName string, headerText string) (headers []sip.Header, err error) {
	var expires sip.Expires
	var value uint64
	value, err = strconv.ParseUint(strings.TrimSpace(headerText), 10, 32)
	expires = sip.Expires(value)
	headers = []sip.Header{&expires}

	return
}

func parseUserAgent(headerName string, headerText string) (headers []sip.Header, err error) {
	var userAgent sip.UserAgentHeader
	headerText = strings.TrimSpace(headerText)
	userAgent = sip.UserAgentHeader(headerText)
	headers = []sip.Header{&userAgent}

	return
}

func parseContentType(headerName string, headerText string) (headers []sip.Header, err error) {
	var contentType sip.ContentType
	headerText = strings.TrimSpace(headerText)
	contentType = sip.ContentType(headerText)
	headers = []sip.Header{&contentType}

	return
}

func parseAccept(headerName string, headerText string) (headers []sip.Header, err error) {
	var accept sip.Accept
	headerText = strings.TrimSpace(headerText)
	accept = sip.Accept(headerText)
	headers = []sip.Header{&accept}

	return
}

func parseAllow(headerName string, headerText string) (headers []sip.Header, err error) {
	allow := make(sip.AllowHeader, 0)
	methods := strings.Split(headerText, ",")
	for _, method := range methods {
		allow = append(allow, sip.RequestMethod(strings.TrimSpace(method)))
	}
	headers = []sip.Header{allow}

	return
}

func parseRequire(headerName string, headerText string) (headers []sip.Header, err error) {
	var require sip.RequireHeader
	require.Options = make([]string, 0)
	extensions := strings.Split(headerText, ",")
	for _, ext := range extensions {
		require.Options = append(require.Options, strings.TrimSpace(ext))
	}
	headers = []sip.Header{&require}

	return
}

func parseSupported(headerName string, headerText string) (headers []sip.Header, err error) {
	var supported sip.SupportedHeader
	supported.Options = make([]string, 0)
	extensions := strings.Split(headerText, ",")
	for _, ext := range extensions {
		supported.Options = append(supported.Options, strings.TrimSpace(ext))
	}
	headers = []sip.Header{&supported}

	return
}

// Parse a string representation of a Content-Length header into a slice of at most one ContentLength header object.
func parseContentLength(headerName string, headerText string) (
	headers []sip.Header, err error) {
	var contentLength sip.ContentLength
	var value uint64
	value, err = strconv.ParseUint(strings.TrimSpace(headerText), 10, 32)
	contentLength = sip.ContentLength(value)

	headers = []sip.Header{&contentLength}
	return
}

// ParseAddressValues parses a comma-separated list of addresses, returning
// any display names and header params, as well as the SIP URIs themselves.
// ParseAddressValues is aware of < > bracketing and quoting, and will not
// break on commas within these structures.
func ParseAddressValues(addresses string) (
	displayNames []sip.MaybeString,
	uris []sip.Uri,
	headerParams []sip.Params,
	err error,
) {

	prevIdx := 0
	inBrackets := false
	inQuotes := false

	// Append a comma to simplify the parsing code; we split address sections
	// on commas, so use a comma to signify the end of the final address section.
	addresses = addresses + ","

	for idx, char := range addresses {
		if char == '<' && !inQuotes {
			inBrackets = true
		} else if char == '>' && !inQuotes {
			inBrackets = false
		} else if char == '"' {
			inQuotes = !inQuotes
		} else if !inQuotes && !inBrackets && char == ',' {
			var displayName sip.MaybeString
			var uri sip.Uri
			var params sip.Params
			displayName, uri, params, err = ParseAddressValue(addresses[prevIdx:idx])
			if err != nil {
				return
			}
			prevIdx = idx + 1

			displayNames = append(displayNames, displayName)
			uris = append(uris, uri)
			headerParams = append(headerParams, params)
		}
	}

	return
}

// ParseAddressValue parses an address - such as from a From, To, or
// Contact header. It returns:
//   - a MaybeString containing the display name (or not)
//   - a parsed SipUri object
//   - a map containing any header parameters present
//   - the error object
//
// See RFC 3261 section 20.10 for details on parsing an address.
// Note that this method will not accept a comma-separated list of addresses;
// addresses in that form should be handled by ParseAddressValues.
func ParseAddressValue(addressText string) (
	displayName sip.MaybeString,
	uri sip.Uri,
	headerParams sip.Params,
	err error,
) {

	headerParams = sip.NewParams(sip.HeaderParams)

	if len(addressText) == 0 {
		err = fmt.Errorf("address-type header has empty body")
		return
	}

	addressTextCopy := addressText
	addressText = strings.TrimSpace(addressText)

	firstAngleBracket := findUnescaped(addressText, '<', quotesDelim)
	displayName = nil
	if firstAngleBracket > 0 {
		// We have an angle bracket, and it's not the first character.
		// Since we have just trimmed whitespace, this means there must
		// be a display name.
		if addressText[0] == '"' {
			// The display name is within quotations.
			// So it comprises all text until the closing quote.
			addressText = addressText[1:]
			nextQuote := strings.Index(addressText, "\"")

			if nextQuote == -1 {
				// Unclosed quotes - parse error.
				err = fmt.Errorf("unclosed quotes in header text: %s",
					addressTextCopy)
				return
			}

			nameField := addressText[:nextQuote]
			displayName = sip.String{Str: nameField}
			addressText = addressText[nextQuote+1:]
		} else {
			// The display name is unquoted, so it comprises
			// all text until the opening angle bracket, except surrounding whitespace.
			// According to the ABNF grammar: display-name   =  *(token LWS)/ quoted-string
			// there are certain characters the display name cannot contain unless it's quoted,
			// however we don't check for them here since it doesn't impact parsing.
			// May as well be lenient.
			nameField := addressText[:firstAngleBracket]
			displayName = sip.String{Str: strings.TrimSpace(nameField)}
			addressText = addressText[firstAngleBracket:]
		}
	}

	// Work out where the SIP URI starts and ends.
	addressText = strings.TrimSpace(addressText)
	var endOfUri int
	var startOfParams int
	if addressText[0] != '<' {
		if displayName != nil {
			// The address must be in <angle brackets> if a display name is
			// present, so this is an invalid address line.
			err = fmt.Errorf(
				"invalid character '%c' following display "+
					"name in address line; expected '<': %s",
				addressText[0],
				addressTextCopy,
			)
			return
		}

		endOfUri = strings.Index(addressText, ";")
		if endOfUri == -1 {
			endOfUri = len(addressText)
		}
		startOfParams = endOfUri

	} else {
		addressText = addressText[1:]
		endOfUri = strings.Index(addressText, ">")
		if endOfUri == 0 {
			err = fmt.Errorf("'<' without closing '>' in address %s",
				addressTextCopy)
			return
		}
		startOfParams = endOfUri + 1

	}

	// Now parse the SIP URI.
	uri, err = ParseUri(addressText[:endOfUri])
	if err != nil {
		return
	}

	if startOfParams >= len(addressText) {
		return
	}

	// Finally, parse any header parameters and then return.
	addressText = addressText[startOfParams:]
	headerParams, _, err = ParseHeaderParams(addressText, ';', ';', ',', true, true)
	return
}

func parseRouteHeader(headerName string, headerText string) (headers []sip.Header, err error) {
	var routeHeader sip.RouteHeader
	routeHeader.Addresses = make([]sip.Uri, 0)
	if _, uris, _, err := ParseAddressValues(headerText); err == nil {
		routeHeader.Addresses = uris
	} else {
		return nil, err
	}
	return []sip.Header{&routeHeader}, nil
}

func parseRecordRouteHeader(headerName string, headerText string) (headers []sip.Header, err error) {
	var routeHeader sip.RecordRouteHeader
	routeHeader.Addresses = make([]sip.Uri, 0)
	if _, uris, _, err := ParseAddressValues(headerText); err == nil {
		routeHeader.Addresses = uris
	} else {
		return nil, err
	}
	return []sip.Header{&routeHeader}, nil
}

// GetNextHeaderLine extract the next logical header line from the message.
// This may run over several actual lines; lines that start with whitespace are
// a continuation of the previous line.
// Therefore also return how many lines we consumed so the parent StreamParser can
// keep track of progress through the message.
func GetNextHeaderLine(contents []string) (headerText string, consumed int) {
	if len(contents) == 0 {
		return
	}
	if len(contents[0]) == 0 {
		return
	}

	var buffer bytes.Buffer
	buffer.WriteString(contents[0])

	for consumed = 1; consumed < len(contents); consumed++ {
		firstChar, _ := utf8.DecodeRuneInString(contents[consumed])
		if !unicode.IsSpace(firstChar) {
			break
		} else if len(contents[consumed]) == 0 {
			break
		}

		buffer.WriteString(" " + strings.TrimSpace(contents[consumed]))
	}

	headerText = buffer.String()
	return
}

// A delimiter is any pair of characters used for quoting text (i.e. bulk escaping literals).
type delimiter struct {
	start uint8
	end   uint8
}

// Define common quote characters needed in parsing.
var quotesDelim = delimiter{'"', '"'}

var anglesDelim = delimiter{'<', '>'}

// Find the first instance of the target in the given text which is not enclosed in any delimiters
// from the list provided.
func findUnescaped(text string, target uint8, delims ...delimiter) int {
	return findAnyUnescaped(text, string(target), delims...)
}

// Find the first instance of the targets in the given text that are not enclosed in any delimiters
// from the list provided.
func findAnyUnescaped(text string, targets string, delims ...delimiter) int {
	escaped := false
	var endEscape uint8 = 0

	endChars := make(map[uint8]uint8)
	for _, delim := range delims {
		endChars[delim.start] = delim.end
	}

	for idx := 0; idx < len(text); idx++ {
		if !escaped && strings.Contains(targets, string(text[idx])) {
			return idx
		}

		if escaped {
			escaped = text[idx] != endEscape
			continue
		} else {
			endEscape, escaped = endChars[text[idx]]
		}
	}

	return -1
}

// SplitByWhitespace splits the given string into sections, separated by one or more characters
// from c_ABNF_WS.
func SplitByWhitespace(text string) []string {
	var buffer bytes.Buffer
	var inString = true
	result := make([]string, 0)

	for _, char := range text {
		s := string(char)
		if strings.Contains(abnfWs, s) {
			if inString {
				// First whitespace char following text; flush buffer to the results array.
				result = append(result, buffer.String())
				buffer.Reset()
			}
			inString = false
		} else {
			buffer.WriteString(s)
			inString = true
		}
	}

	if buffer.Len() > 0 {
		result = append(result, buffer.String())
	}

	return result
}
