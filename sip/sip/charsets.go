package sip

import (
	"bytes"
	"fmt"
)

const (
	hexChars = "0123456789abcdef"
)

var (
	tokencMask  [2]uint64
	qdtextcMask [2]uint64
	usercMask   [2]uint64
	passcMask   [2]uint64
	paramcMask  [2]uint64
	headercMask [2]uint64
)

func tokenc(c byte) bool {
	return charsetContains(&tokencMask, c)
}

func qdtextc(c byte) bool {
	return charsetContains(&qdtextcMask, c)
}

func userc(c byte) bool {
	return charsetContains(&usercMask, c)
}

func passc(c byte) bool {
	return charsetContains(&passcMask, c)
}

func paramc(c byte) bool {
	return charsetContains(&paramcMask, c)
}

func headerc(c byte) bool {
	return charsetContains(&headercMask, c)
}

func qdtextesc(c byte) bool {
	return c <= 0x09 ||
		0x0B <= c && c <= 0x0C ||
		0x0E <= c && c <= 0x7F
}

func whitespacec(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func QuotedString(s string) string {
	return fmt.Sprintf(`"%s"`, s)
}

func init() {
	charsetAddAlphaNumeric(&tokencMask)
	charsetAdd(&tokencMask, '-', '.', '!', '%', '*', '_', '+', '`', '\'', '~')

	charsetAddRange(&qdtextcMask, 0x23, 0x5B)
	charsetAddRange(&qdtextcMask, 0x5D, 0x7E)
	charsetAdd(&qdtextcMask, '\r', '\n', '\t', ' ', '!')

	charsetAddAlphaNumeric(&usercMask)
	charsetAddMark(&usercMask)
	charsetAdd(&usercMask, '&', '=', '+', '$', ',', ';', '?', '/')

	charsetAddAlphaNumeric(&passcMask)
	charsetAddMark(&passcMask)
	charsetAdd(&passcMask, '&', '=', '+', '$', ',')

	charsetAddAlphaNumeric(&paramcMask)
	charsetAddMark(&paramcMask)
	charsetAdd(&paramcMask, '[', ']', '/', ':', '&', '+', '$')

	charsetAddAlphaNumeric(&headercMask)
	charsetAddMark(&headercMask)
	charsetAdd(&headercMask, '[', ']', '/', '?', ':', '+', '$')
}

func charsetContains(mask *[2]uint64, i byte) bool {
	return i < 128 && mask[i/64]&(1<<(i%64)) != 0
}

func charsetAdd(mask *[2]uint64, vi ...byte) {
	for _, i := range vi {
		mask[i/64] |= 1 << (i % 64)
	}
}

func charsetAddRange(mask *[2]uint64, a, b byte) {
	for i := a; i <= b; i++ {
		charsetAdd(mask, i)
	}
}

func charsetAddMark(mask *[2]uint64) {
	charsetAdd(mask, '-', '_', '.', '!', '~', '*', '\'', '(', ')')
}

func charsetAddAlphaNumeric(mask *[2]uint64) {
	charsetAddRange(mask, 'a', 'z')
	charsetAddRange(mask, 'A', 'Z')
	charsetAddRange(mask, '0', '9')
}

// appendEscaped appends while URL encoding bytes that don't match the predicate..
func appendEscaped(b *bytes.Buffer, s []byte, p func(byte) bool) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if p(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hexChars[c>>4])
			b.WriteByte(hexChars[c%16])
		}
	}
}

// appendSanitized appends stripping all characters that don't match the predicate.
func appendSanitized(b *bytes.Buffer, s []byte, p func(byte) bool) {
	for i := 0; i < len(s); i++ {
		if p(s[i]) {
			b.WriteByte(s[i])
		}
	}
}

// quote formats an address parameter value or display name.
//
// Quotation marks are only added if necessary. This implementation will
// truncate on input error.
func appendQuoted(b *bytes.Buffer, s []byte) {
	for i := 0; i < len(s); i++ {
		if !tokenc(s[i]) && s[i] != ' ' {
			appendQuoteQuoted(b, s)
			return
		}
	}
	b.Write(s)
}

// quoteQuoted formats an address parameter value or display name with quotes.
//
// This implementation will truncate on input error.
func appendQuoteQuoted(b *bytes.Buffer, s []byte) {
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		if qdtextc(s[i]) {
			if s[i] == '\r' {
				if i+2 >= len(s) || s[i+1] != '\n' ||
					!(s[i+2] == ' ' || s[i+2] == '\t') {
					break
				}
			}
		} else {
			if !qdtextesc(s[i]) {
				break
			}
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')
}
