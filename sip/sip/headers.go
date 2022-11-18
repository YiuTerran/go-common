package sip

import (
	"bytes"
	"fmt"
	"github.com/YiuTerran/go-common/base/util/ptrutil"
	"strings"
)

// SIP Headers structs
// Originally forked from github.com/stefankopieczek/gossip by @StefanKopieczek
// with a tiny changes

// Header is a single SIP header.
type Header interface {
	// Name returns header name.
	Name() string
	Value() string
	// Clone returns copy of header struct.
	Clone() Header
	String() string
	Equals(other any) bool
}

// GenericHeader encapsulates a header that gossip does not natively support.
// This allows header data that is not understood to be parsed by gossip and relayed to the parent application.
type GenericHeader struct {
	// The name of the header.
	HeaderName string
	// The contents of the header, including any parameters.
	// This is transparent data that is not natively understood by gossip.
	Contents string
}

// Convert the header to a flat string representation.
func (header *GenericHeader) String() string {
	return header.HeaderName + ": " + header.Contents
}

// Name pull out the header name.
func (header *GenericHeader) Name() string {
	return header.HeaderName
}

func (header *GenericHeader) Value() string {
	return header.Contents
}

// Clone the header.
func (header *GenericHeader) Clone() Header {
	if header == nil {
		var newHeader *GenericHeader
		return newHeader
	}

	return &GenericHeader{
		HeaderName: header.HeaderName,
		Contents:   header.Contents,
	}
}

func (header *GenericHeader) Equals(other any) bool {
	if h, ok := other.(*GenericHeader); ok {
		if header == h {
			return true
		}
		if header == nil && h != nil || header != nil && h == nil {
			return false
		}

		return header.HeaderName == h.HeaderName &&
			header.Contents == h.Contents
	}

	return false
}

// ToHeader introduces SIP 'To' header
type ToHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString
	Address     Uri
	// Any parameters present in the header.
	Params Params
}

func (to *ToHeader) String() string {
	return fmt.Sprintf("%s: %s", to.Name(), to.Value())
}

func (to *ToHeader) Name() string { return "To" }

func (to *ToHeader) Value() string {
	var buffer bytes.Buffer
	if displayName, ok := to.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", to.Address))

	if to.Params != nil && to.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(to.Params.String())
	}

	return buffer.String()
}

// Clone the header.
func (to *ToHeader) Clone() Header {
	var newTo *ToHeader
	if to == nil {
		return newTo
	}

	newTo = &ToHeader{
		DisplayName: to.DisplayName,
	}
	if to.Address != nil {
		newTo.Address = to.Address.Clone()
	}
	if to.Params != nil {
		newTo.Params = to.Params.Clone()
	}
	return newTo
}

func (to *ToHeader) Equals(other any) bool {
	if h, ok := other.(*ToHeader); ok {
		if to == h {
			return true
		}
		if to == nil && h != nil || to != nil && h == nil {
			return false
		}

		res := true

		if to.DisplayName != h.DisplayName {
			if to.DisplayName == nil {
				res = res && h.DisplayName == nil
			} else {
				res = res && to.DisplayName.Equals(h.DisplayName)
			}
		}

		if to.Address != h.Address {
			if to.Address == nil {
				res = res && h.Address == nil
			} else {
				res = res && to.Address.Equals(h.Address)
			}
		}

		if to.Params != h.Params {
			if to.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && to.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

type FromHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString

	Address Uri

	// Any parameters present in the header.
	Params Params
}

func (from *FromHeader) String() string {
	return fmt.Sprintf("%s: %s", from.Name(), from.Value())
}

func (from *FromHeader) Name() string { return "From" }

func (from *FromHeader) Value() string {
	var buffer bytes.Buffer
	if displayName, ok := from.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", from.Address))

	if from.Params != nil && from.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(from.Params.String())
	}

	return buffer.String()
}

// Clone the header.
func (from *FromHeader) Clone() Header {
	var newFrom *FromHeader
	if from == nil {
		return newFrom
	}

	newFrom = &FromHeader{
		DisplayName: from.DisplayName,
	}
	if from.Address != nil {
		newFrom.Address = from.Address.Clone()
	}
	if from.Params != nil {
		newFrom.Params = from.Params.Clone()
	}

	return newFrom
}

func (from *FromHeader) Equals(other any) bool {
	if h, ok := other.(*FromHeader); ok {
		if from == h {
			return true
		}
		if from == nil && h != nil || from != nil && h == nil {
			return false
		}

		res := true

		if from.DisplayName != h.DisplayName {
			if from.DisplayName == nil {
				res = res && h.DisplayName == nil
			} else {
				res = res && from.DisplayName.Equals(h.DisplayName)
			}
		}

		if from.Address != h.Address {
			if from.Address == nil {
				res = res && h.Address == nil
			} else {
				res = res && from.Address.Equals(h.Address)
			}
		}

		if from.Params != h.Params {
			if from.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && from.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

type ContactHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString
	Address     ContactUri
	// Any parameters present in the header.
	Params Params
}

func (contact *ContactHeader) String() string {
	return fmt.Sprintf("%s: %s", contact.Name(), contact.Value())
}

func (contact *ContactHeader) Name() string { return "Contact" }

func (contact *ContactHeader) Value() string {
	var buffer bytes.Buffer

	if displayName, ok := contact.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	switch contact.Address.(type) {
	case *WildcardUri:
		// Treat the Wildcard URI separately as it must not be contained in < > angle brackets.
		buffer.WriteString("*")
	default:
		buffer.WriteString(fmt.Sprintf("<%s>", contact.Address.String()))
	}

	if (contact.Params != nil) && (contact.Params.Length() > 0) {
		buffer.WriteString(";")
		buffer.WriteString(contact.Params.String())
	}

	return buffer.String()
}

// Clone the header.
func (contact *ContactHeader) Clone() Header {
	var newCnt *ContactHeader
	if contact == nil {
		return newCnt
	}

	newCnt = &ContactHeader{
		DisplayName: contact.DisplayName,
	}
	if contact.Address != nil {
		newCnt.Address = contact.Address.Clone()
	}
	if contact.Params != nil {
		newCnt.Params = contact.Params.Clone()
	}

	return newCnt
}

func (contact *ContactHeader) Equals(other any) bool {
	if h, ok := other.(*ContactHeader); ok {
		if contact == h {
			return true
		}
		if contact == nil && h != nil || contact != nil && h == nil {
			return false
		}

		res := true

		if contact.DisplayName != h.DisplayName {
			if contact.DisplayName == nil {
				res = res && h.DisplayName == nil
			} else {
				res = res && contact.DisplayName.Equals(h.DisplayName)
			}
		}

		if contact.Address != h.Address {
			if contact.Address == nil {
				res = res && h.Address == nil
			} else {
				res = res && contact.Address.Equals(h.Address)
			}
		}

		if contact.Params != h.Params {
			if contact.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && contact.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

// CallID - 'Call-ID' header.
type CallID string

func (callId CallID) String() string {
	return fmt.Sprintf("%s: %s", callId.Name(), callId.Value())
}

func (callId *CallID) Name() string { return "Call-ID" }

func (callId CallID) Value() string { return string(callId) }

func (callId *CallID) Clone() Header {
	return callId
}

func (callId *CallID) Equals(other any) bool {
	if h, ok := other.(CallID); ok {
		if callId == nil {
			return false
		}

		return *callId == h
	}
	if h, ok := other.(*CallID); ok {
		if callId == h {
			return true
		}
		if callId == nil && h != nil || callId != nil && h == nil {
			return false
		}

		return *callId == *h
	}

	return false
}

type CSeq struct {
	SeqNo      uint32
	MethodName RequestMethod
}

func (cseq *CSeq) String() string {
	return fmt.Sprintf("%s: %s", cseq.Name(), cseq.Value())
}

func (cseq *CSeq) Name() string { return "CSeq" }

func (cseq *CSeq) Value() string {
	return fmt.Sprintf("%d %s", cseq.SeqNo, cseq.MethodName)
}

func (cseq *CSeq) Clone() Header {
	if cseq == nil {
		var newCSeq *CSeq
		return newCSeq
	}

	return &CSeq{
		SeqNo:      cseq.SeqNo,
		MethodName: cseq.MethodName,
	}
}

func (cseq *CSeq) Equals(other any) bool {
	if h, ok := other.(*CSeq); ok {
		if cseq == h {
			return true
		}
		if cseq == nil && h != nil || cseq != nil && h == nil {
			return false
		}

		return cseq.SeqNo == h.SeqNo &&
			cseq.MethodName == h.MethodName
	}

	return false
}

type MaxForwards uint32

func (maxForwards MaxForwards) String() string {
	return fmt.Sprintf("%s: %s", maxForwards.Name(), maxForwards.Value())
}

func (maxForwards *MaxForwards) Name() string { return "Max-Forwards" }

func (maxForwards MaxForwards) Value() string { return fmt.Sprintf("%d", maxForwards) }

func (maxForwards *MaxForwards) Clone() Header { return maxForwards }

func (maxForwards *MaxForwards) Equals(other any) bool {
	if h, ok := other.(MaxForwards); ok {
		if maxForwards == nil {
			return false
		}

		return *maxForwards == h
	}
	if h, ok := other.(*MaxForwards); ok {
		if maxForwards == h {
			return true
		}
		if maxForwards == nil && h != nil || maxForwards != nil && h == nil {
			return false
		}

		return *maxForwards == *h
	}

	return false
}

type Expires uint32

func (expires *Expires) String() string {
	return fmt.Sprintf("%s: %s", expires.Name(), expires.Value())
}

func (expires *Expires) Name() string { return "Expires" }

func (expires Expires) Value() string { return fmt.Sprintf("%d", expires) }

func (expires *Expires) Clone() Header { return expires }

func (expires *Expires) Equals(other any) bool {
	if h, ok := other.(Expires); ok {
		if expires == nil {
			return false
		}

		return *expires == h
	}
	if h, ok := other.(*Expires); ok {
		if expires == h {
			return true
		}
		if expires == nil && h != nil || expires != nil && h == nil {
			return false
		}

		return *expires == *h
	}

	return false
}

type ContentLength uint32

func (contentLength ContentLength) String() string {
	return fmt.Sprintf("%s: %s", contentLength.Name(), contentLength.Value())
}

func (contentLength *ContentLength) Name() string { return "Content-Length" }

func (contentLength ContentLength) Value() string { return fmt.Sprintf("%d", contentLength) }

func (contentLength *ContentLength) Clone() Header { return contentLength }

func (contentLength *ContentLength) Equals(other any) bool {
	if h, ok := other.(ContentLength); ok {
		if contentLength == nil {
			return false
		}

		return *contentLength == h
	}
	if h, ok := other.(*ContentLength); ok {
		if contentLength == h {
			return true
		}
		if contentLength == nil && h != nil || contentLength != nil && h == nil {
			return false
		}

		return *contentLength == *h
	}

	return false
}

type ViaHeader []*ViaHop

func (via ViaHeader) String() string {
	return fmt.Sprintf("%s: %s", via.Name(), via.Value())
}

func (via ViaHeader) Name() string { return "Via" }

func (via ViaHeader) Value() string {
	var buffer bytes.Buffer
	for idx, hop := range via {
		buffer.WriteString(hop.String())
		if idx != len(via)-1 {
			buffer.WriteString(", ")
		}
	}

	return buffer.String()
}

func (via ViaHeader) Clone() Header {
	if via == nil {
		var newVie ViaHeader
		return newVie
	}

	dup := make([]*ViaHop, 0, len(via))
	for _, hop := range via {
		dup = append(dup, hop.Clone())
	}
	return ViaHeader(dup)
}

func (via ViaHeader) Equals(other any) bool {
	if h, ok := other.(ViaHeader); ok {
		if len(via) != len(h) {
			return false
		}

		for i, hop := range via {
			if !hop.Equals(h[i]) {
				return false
			}
		}

		return true
	}

	return false
}

// ViaHop is a single component for a Via header.
// Via headers are composed of several segments of the same structure, added by successive nodes in a routing chain.
type ViaHop struct {
	// E.g. 'SIP'.
	ProtocolName string
	// E.g. '2.0'.
	ProtocolVersion string
	Transport       string
	Host            string
	// The port for this via hop. This is stored as a pointer type, since it is an optional field.
	Port   *Port
	Params Params
}

func (hop *ViaHop) SentBy() string {
	var buf bytes.Buffer
	buf.WriteString(hop.Host)
	if hop.Port != nil {
		buf.WriteString(fmt.Sprintf(":%d", *hop.Port))
	}

	return buf.String()
}

func (hop *ViaHop) String() string {
	var buffer bytes.Buffer
	buffer.WriteString(
		fmt.Sprintf(
			"%s/%s/%s %s",
			hop.ProtocolName,
			hop.ProtocolVersion,
			hop.Transport,
			hop.Host,
		),
	)
	if hop.Port != nil {
		buffer.WriteString(fmt.Sprintf(":%d", *hop.Port))
	}

	if hop.Params != nil && hop.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(hop.Params.String())
	}

	return buffer.String()
}

// Clone return an exact copy of this ViaHop.
func (hop *ViaHop) Clone() *ViaHop {
	var newHop *ViaHop
	if hop == nil {
		return newHop
	}

	newHop = &ViaHop{
		ProtocolName:    hop.ProtocolName,
		ProtocolVersion: hop.ProtocolVersion,
		Transport:       hop.Transport,
		Host:            hop.Host,
	}
	if hop.Port != nil {
		newHop.Port = hop.Port.Clone()
	}
	if hop.Params != nil {
		newHop.Params = hop.Params.Clone()
	}

	return newHop
}

func (hop *ViaHop) Equals(other any) bool {
	if h, ok := other.(*ViaHop); ok {
		if hop == h {
			return true
		}
		if hop == nil && h != nil || hop != nil && h == nil {
			return false
		}

		res := hop.ProtocolName == h.ProtocolName &&
			hop.ProtocolVersion == h.ProtocolVersion &&
			hop.Transport == h.Transport &&
			hop.Host == h.Host &&
			ptrutil.Equal((*uint16)(hop.Port), (*uint16)(h.Port))

		if hop.Params != h.Params {
			if hop.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && hop.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

type RequireHeader struct {
	Options []string
}

func (require *RequireHeader) String() string {
	return fmt.Sprintf("%s: %s", require.Name(), require.Value())
}

func (require *RequireHeader) Name() string { return "Require" }

func (require *RequireHeader) Value() string {
	return strings.Join(require.Options, ", ")
}

func (require *RequireHeader) Clone() Header {
	if require == nil {
		var newRequire *RequireHeader
		return newRequire
	}

	dup := make([]string, len(require.Options))
	copy(dup, require.Options)
	return &RequireHeader{dup}
}

func (require *RequireHeader) Equals(other any) bool {
	if h, ok := other.(*RequireHeader); ok {
		if require == h {
			return true
		}
		if require == nil && h != nil || require != nil && h == nil {
			return false
		}

		if len(require.Options) != len(h.Options) {
			return false
		}

		for i, opt := range require.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

type SupportedHeader struct {
	Options []string
}

func (support *SupportedHeader) String() string {
	return fmt.Sprintf("%s: %s", support.Name(), support.Value())
}

func (support *SupportedHeader) Name() string { return "Supported" }

func (support *SupportedHeader) Value() string {
	return strings.Join(support.Options, ", ")
}

func (support *SupportedHeader) Clone() Header {
	if support == nil {
		var newSupport *SupportedHeader
		return newSupport
	}

	dup := make([]string, len(support.Options))
	copy(dup, support.Options)
	return &SupportedHeader{dup}
}

func (support *SupportedHeader) Equals(other any) bool {
	if h, ok := other.(*SupportedHeader); ok {
		if support == h {
			return true
		}
		if support == nil && h != nil || support != nil && h == nil {
			return false
		}

		if len(support.Options) != len(h.Options) {
			return false
		}

		for i, opt := range support.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

type ProxyRequireHeader struct {
	Options []string
}

func (proxyRequire *ProxyRequireHeader) String() string {
	return fmt.Sprintf("%s: %s", proxyRequire.Name(), proxyRequire.Value())
}

func (proxyRequire *ProxyRequireHeader) Name() string { return "Proxy-Require" }

func (proxyRequire *ProxyRequireHeader) Value() string {
	return strings.Join(proxyRequire.Options, ", ")
}

func (proxyRequire *ProxyRequireHeader) Clone() Header {
	if proxyRequire == nil {
		var newProxy *ProxyRequireHeader
		return newProxy
	}

	dup := make([]string, len(proxyRequire.Options))
	copy(dup, proxyRequire.Options)
	return &ProxyRequireHeader{dup}
}

func (proxyRequire *ProxyRequireHeader) Equals(other any) bool {
	if h, ok := other.(*ProxyRequireHeader); ok {
		if proxyRequire == h {
			return true
		}
		if proxyRequire == nil && h != nil || proxyRequire != nil && h == nil {
			return false
		}

		if len(proxyRequire.Options) != len(h.Options) {
			return false
		}

		for i, opt := range proxyRequire.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

// UnsupportedHeader is a SIP header type named 'Unsupported'
//this doesn't indicate that the header itself is not supported by gossip!
type UnsupportedHeader struct {
	Options []string
}

func (unsupported *UnsupportedHeader) String() string {
	return fmt.Sprintf("%s: %s", unsupported.Name(), unsupported.Value())
}

func (unsupported *UnsupportedHeader) Name() string { return "Unsupported" }

func (unsupported *UnsupportedHeader) Value() string {
	return strings.Join(unsupported.Options, ", ")
}

func (unsupported *UnsupportedHeader) Clone() Header {
	if unsupported == nil {
		var newUnsup *UnsupportedHeader
		return newUnsup
	}

	dup := make([]string, len(unsupported.Options))
	copy(dup, unsupported.Options)
	return &UnsupportedHeader{dup}
}

func (unsupported *UnsupportedHeader) Equals(other any) bool {
	if h, ok := other.(*UnsupportedHeader); ok {
		if unsupported == h {
			return true
		}
		if unsupported == nil && h != nil || unsupported != nil && h == nil {
			return false
		}

		if len(unsupported.Options) != len(h.Options) {
			return false
		}

		for i, opt := range unsupported.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

type UserAgentHeader string

func (ua *UserAgentHeader) String() string {
	return fmt.Sprintf("%s: %s", ua.Name(), ua.Value())
}

func (ua *UserAgentHeader) Name() string { return "User-Agent" }

func (ua UserAgentHeader) Value() string { return string(ua) }

func (ua *UserAgentHeader) Clone() Header { return ua }

func (ua *UserAgentHeader) Equals(other any) bool {
	if h, ok := other.(UserAgentHeader); ok {
		if ua == nil {
			return false
		}

		return *ua == h
	}
	if h, ok := other.(*UserAgentHeader); ok {
		if ua == h {
			return true
		}
		if ua == nil && h != nil || ua != nil && h == nil {
			return false
		}

		return *ua == *h
	}

	return false
}

type AllowHeader []RequestMethod

func (allow AllowHeader) String() string {
	return fmt.Sprintf("%s: %s", allow.Name(), allow.Value())
}

func (allow AllowHeader) Name() string { return "Allow" }

func (allow AllowHeader) Value() string {
	parts := make([]string, 0)
	for _, method := range allow {
		parts = append(parts, string(method))
	}
	return strings.Join(parts, ", ")
}

func (allow AllowHeader) Clone() Header {
	if allow == nil {
		var newAllow AllowHeader
		return newAllow
	}

	newAllow := make(AllowHeader, len(allow))
	copy(newAllow, allow)

	return newAllow
}

func (allow AllowHeader) Equals(other any) bool {
	if h, ok := other.(AllowHeader); ok {
		if len(allow) != len(h) {
			return false
		}

		for i, v := range allow {
			if v != h[i] {
				return false
			}
		}

		return true
	}

	return false
}

type ContentType string

func (ct *ContentType) String() string { return fmt.Sprintf("%s: %s", ct.Name(), ct.Value()) }

func (ct *ContentType) Name() string { return "Content-Type" }

func (ct ContentType) Value() string { return string(ct) }

func (ct *ContentType) Clone() Header { return ct }

func (ct *ContentType) Equals(other any) bool {
	if h, ok := other.(ContentType); ok {
		if ct == nil {
			return false
		}

		return *ct == h
	}
	if h, ok := other.(*ContentType); ok {
		if ct == h {
			return true
		}
		if ct == nil && h != nil || ct != nil && h == nil {
			return false
		}

		return *ct == *h
	}

	return false
}

type Accept string

func (ct *Accept) String() string { return fmt.Sprintf("%s: %s", ct.Name(), ct.Value()) }

func (ct *Accept) Name() string { return "Accept" }

func (ct Accept) Value() string { return string(ct) }

func (ct *Accept) Clone() Header { return ct }

func (ct *Accept) Equals(other any) bool {
	if h, ok := other.(Accept); ok {
		if ct == nil {
			return false
		}

		return *ct == h
	}
	if h, ok := other.(*Accept); ok {
		if ct == h {
			return true
		}
		if ct == nil && h != nil || ct != nil && h == nil {
			return false
		}

		return *ct == *h
	}

	return false
}

type RouteHeader struct {
	Addresses []Uri
}

func (route *RouteHeader) Name() string { return "Route" }

func (route *RouteHeader) Value() string {
	var addrs []string
	for _, uri := range route.Addresses {
		addrs = append(addrs, "<"+uri.String()+">")
	}
	return strings.Join(addrs, ", ")
}

func (route *RouteHeader) String() string {
	return fmt.Sprintf("%s: %s", route.Name(), route.Value())
}

func (route *RouteHeader) Clone() Header {
	var newRoute *RouteHeader
	if route == nil {
		return newRoute
	}

	newRoute = &RouteHeader{
		Addresses: make([]Uri, len(route.Addresses)),
	}

	for i, uri := range route.Addresses {
		newRoute.Addresses[i] = uri.Clone()
	}

	return newRoute
}

func (route *RouteHeader) Equals(other any) bool {
	if h, ok := other.(*RouteHeader); ok {
		if route == h {
			return true
		}
		if route == nil && h != nil || route != nil && h == nil {
			return false
		}

		for i, uri := range route.Addresses {
			if !uri.Equals(h.Addresses[i]) {
				return false
			}
		}

		return true
	}

	return false
}

type RecordRouteHeader struct {
	Addresses []Uri
}

func (route *RecordRouteHeader) Name() string { return "Record-Route" }

func (route *RecordRouteHeader) Value() string {
	var addrs []string
	for _, uri := range route.Addresses {
		addrs = append(addrs, "<"+uri.String()+">")
	}
	return strings.Join(addrs, ", ")
}

func (route *RecordRouteHeader) String() string {
	return fmt.Sprintf("%s: %s", route.Name(), route.Value())
}

func (route *RecordRouteHeader) Clone() Header {
	var newRoute *RecordRouteHeader
	if route == nil {
		return newRoute
	}

	newRoute = &RecordRouteHeader{
		Addresses: make([]Uri, len(route.Addresses)),
	}

	for i, uri := range route.Addresses {
		newRoute.Addresses[i] = uri.Clone()
	}

	return newRoute
}

func (route *RecordRouteHeader) Equals(other any) bool {
	if h, ok := other.(*RecordRouteHeader); ok {
		if route == h {
			return true
		}
		if route == nil && h != nil || route != nil && h == nil {
			return false
		}

		for i, uri := range route.Addresses {
			if !uri.Equals(h.Addresses[i]) {
				return false
			}
		}

		return true
	}

	return false
}
