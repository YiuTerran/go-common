package sip

import (
	"bytes"
	"fmt"
	"github.com/YiuTerran/go-common/base/util/ptrutil"
	"strings"
)

/**
  *  @author tryao
  *  @date 2022/03/25 16:52
**/

// Uri from any schema (e.g. sip:, tel:, callto:)
type Uri interface {
	// Equals Determine if the two URIs are equal according to the rules in RFC 3261 s. 19.1.4.
	Equals(other any) bool
	String() string
	Clone() Uri

	IsEncrypted() bool
	SetEncrypted(flag bool)
	User() MaybeString
	SetUser(user MaybeString)
	Password() MaybeString
	SetPassword(pass MaybeString)
	Host() string
	SetHost(host string)
	Port() *Port
	SetPort(port *Port)
	UriParams() Params
	SetUriParams(params Params)
	Headers() Params
	SetHeaders(params Params)
	// IsWildcard Return true if and only if the URI is the special wildcard URI '*'; that is, if it is
	// a WildcardUri struct.
	IsWildcard() bool
}

// ContactUri from a schema suitable for inclusion in a Contact: header.
// The only such URIs are sip/sips URIs and the special wildcard URI '*'.
// hold this interface to not break other code
type ContactUri interface {
	Uri
}

// SipUri represents a SIP or SIPS URI, including all params and URI header params.
// format: sip:user:password@host:port;uri-parameters?headers
type SipUri struct {
	// True if and only if the URI is a SIPS URI.
	FIsEncrypted bool

	// The user part of the URI: the 'joe' in sip:joe@bloggs.com
	// This is a pointer, so that URIs without a user part can have 'nil'.
	FUser MaybeString

	// The password field of the URI. This is represented in the URI as joe:hunter2@bloggs.com.
	// Note that if a URI has a password field, it *must* have a user field as well.
	// This is a pointer, so that URIs without a password field can have 'nil'.
	// Note that RFC 3261 strongly recommends against the use of password fields in SIP URIs,
	// as they are fundamentally insecure.
	FPassword MaybeString

	// The host part of the URI. This can be a domain, or a string representation of an IP address.
	FHost string

	// The port part of the URI. This is optional, and so is represented here as a pointer type.
	FPort *Port

	// Any parameters associated with the URI.
	// These are used to provide information about requests that may be constructed from the URI.
	// (For more details, see RFC 3261 section 19.1.1).
	// These appear as a semicolon-separated list of key=value pairs following the host[:port] part.
	FUriParams Params

	// Any headers to be included on requests constructed from this URI.
	// These appear as a '&'-separated list at the end of the URI, introduced by '?'.
	// Although the values of the map are MaybeStrings, they will never be NoString in practice as the parser
	// guarantees to not return blank values for header elements in SIP URIs.
	// You should not set the values of headers to NoString.
	FHeaders Params
}

func (uri *SipUri) IsEncrypted() bool {
	return uri.FIsEncrypted
}

func (uri *SipUri) SetEncrypted(flag bool) {
	uri.FIsEncrypted = flag
}

func (uri *SipUri) User() MaybeString {
	return uri.FUser
}

func (uri *SipUri) SetUser(user MaybeString) {
	uri.FUser = user
}

func (uri *SipUri) Password() MaybeString {
	return uri.FPassword
}

func (uri *SipUri) SetPassword(pass MaybeString) {
	uri.FPassword = pass
}

func (uri *SipUri) Host() string {
	return uri.FHost
}

func (uri *SipUri) SetHost(host string) {
	uri.FHost = host
}

func (uri *SipUri) Port() *Port {
	return uri.FPort
}

func (uri *SipUri) SetPort(port *Port) {
	uri.FPort = port
}

func (uri *SipUri) UriParams() Params {
	return uri.FUriParams
}

func (uri *SipUri) SetUriParams(params Params) {
	uri.FUriParams = params
}

func (uri *SipUri) Headers() Params {
	return uri.FHeaders
}

func (uri *SipUri) SetHeaders(params Params) {
	uri.FHeaders = params
}

func (uri *SipUri) IsWildcard() bool {
	return false
}

// Equals determine if the SIP URI is equal to the specified URI according to the rules laid down in RFC 3261 s. 19.1.4.
func (uri *SipUri) Equals(val any) bool {
	other, ok := val.(*SipUri)
	if !ok {
		inst, ok := val.(SipUri)
		if ok {
			other = &inst
		} else {
			return false
		}
	}
	if uri == other {
		return true
	}
	if uri == nil || other == nil {
		return uri == other
	}

	if uri.FIsEncrypted != other.FIsEncrypted {
		return false
	}
	if !IsStringEqual(uri.FUser, other.FUser) {
		return false
	}
	if !IsStringEqual(uri.FPassword, other.FPassword) {
		return false
	}
	if !strings.EqualFold(uri.FHost, other.FHost) {
		return false
	}
	if !ptrutil.Equal((*uint16)(uri.FPort), (*uint16)(other.FPort)) {
		return false
	}
	return IsParamsEqual(uri.FUriParams, other.FUriParams) &&
		IsParamsEqual(uri.FHeaders, other.FHeaders)
}

// Generates the string representation of a SipUri struct.
func (uri *SipUri) String() string {
	var buffer bytes.Buffer

	// Compulsory protocol identifier.
	if uri.FIsEncrypted {
		buffer.WriteString("sips")
		buffer.WriteString(":")
	} else {
		buffer.WriteString("sip")
		buffer.WriteString(":")
	}

	// Optional userinfo part.
	if user, ok := uri.FUser.(String); ok && user.String() != "" {
		appendEscaped(&buffer, []byte(uri.FUser.String()), userc)
		if pass, ok := uri.FPassword.(String); ok && pass.String() != "" {
			buffer.WriteString(":")
			appendEscaped(&buffer, []byte(pass.String()), passc)
		}
		buffer.WriteString("@")
	}

	// Compulsory hostname.
	buffer.WriteString(uri.FHost)

	// Optional port number.
	if uri.FPort != nil {
		buffer.WriteString(fmt.Sprintf(":%d", *uri.FPort))
	}

	if (uri.FUriParams != nil) && uri.FUriParams.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(uri.FUriParams.String())
	}

	if (uri.FHeaders != nil) && uri.FHeaders.Length() > 0 {
		buffer.WriteString("?")
		buffer.WriteString(uri.FHeaders.String())
	}

	return buffer.String()
}

// Clone the Sip URI.
func (uri *SipUri) Clone() Uri {
	var newUri *SipUri
	if uri == nil {
		return newUri
	}

	newUri = &SipUri{
		FIsEncrypted: uri.FIsEncrypted,
		FUser:        uri.FUser,
		FPassword:    uri.FPassword,
		FHost:        uri.FHost,
		FUriParams:   cloneWithNil(uri.FUriParams, UriParams),
		FHeaders:     cloneWithNil(uri.FHeaders, UriHeaders),
	}
	if uri.FPort != nil {
		newUri.FPort = uri.FPort.Clone()
	}
	return newUri
}

// WildcardUri is the special wildcard URI used in Contact:
//headers in REGISTER requests when expiring all registrations.
type WildcardUri struct{}

func (uri WildcardUri) IsEncrypted() bool { return false }

func (uri WildcardUri) SetEncrypted(flag bool) {}

func (uri WildcardUri) User() MaybeString { return nil }

func (uri WildcardUri) SetUser(user MaybeString) {}

func (uri WildcardUri) Password() MaybeString { return nil }

func (uri WildcardUri) SetPassword(pass MaybeString) {}

func (uri WildcardUri) Host() string { return "" }

func (uri WildcardUri) SetHost(host string) {}

func (uri WildcardUri) Port() *Port { return nil }

func (uri WildcardUri) SetPort(port *Port) {}

func (uri WildcardUri) UriParams() Params { return nil }

func (uri WildcardUri) SetUriParams(params Params) {}

func (uri WildcardUri) Headers() Params { return nil }

func (uri WildcardUri) SetHeaders(params Params) {}

// Clone copy the wildcard URI. Not hard!
func (uri WildcardUri) Clone() Uri { return &WildcardUri{} }

// IsWildcard always returns 'true'.
func (uri WildcardUri) IsWildcard() bool {
	return true
}

// Always returns '*' - the representation of a wildcard URI in a SIP message.
func (uri WildcardUri) String() string {
	return "*"
}

// Equals determines if this wildcard URI equals the specified other URI.
// This is true if and only if the other URI is also a wildcard URI.
func (uri WildcardUri) Equals(other any) bool {
	switch other.(type) {
	case WildcardUri:
		return true
	default:
		return false
	}
}
