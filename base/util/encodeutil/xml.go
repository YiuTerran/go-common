package encodeutil

import (
	"bytes"
	"encoding/xml"
	"github.com/samber/lo"
	"golang.org/x/net/html/charset"
	"strings"
)

/**	 xml支持gbk编码
  *  @author tryao
  *  @date 2022/04/18 14:32
**/

const (
	GBK    = "gbk"
	GB2312 = "gb2312"
	UTF8   = "UTF-8"
)

var (
	headerMap = map[string][]byte{
		GBK:    []byte(`<?xml version="1.0" encoding="GBK"?>` + "\n"),
		GB2312: []byte(`<?xml version="1.0" encoding="GB2312"?>` + "\n"),
		UTF8:   []byte(xml.Header),
	}
)

func xmlEncode(data any, code string, useHeader bool, intend int) ([]byte, error) {
	var (
		bs  []byte
		err error
	)

	if intend > 0 {
		sp := ""
		for i := 0; i < intend; i++ {
			sp += " "
		}
		bs, err = xml.MarshalIndent(data, "", sp)
	} else {
		bs, err = xml.Marshal(data)
	}
	if err != nil {
		return nil, err
	}
	if code != UTF8 {
		bs, err = Utf8ToGbk(bs)
		if err != nil {
			return nil, err
		}
	}
	if !useHeader {
		return bs, nil
	}
	header := string(headerMap[code])
	if intend <= 0 {
		header = strings.TrimSpace(header)
	}
	return append([]byte(header), bs...), nil
}

func XmlGBKEncode(data any, useHeader, intend bool) ([]byte, error) {
	return xmlEncode(data, GBK, useHeader, lo.Ternary(intend, 2, 0))
}

func XmlGb2312Encode(data any, useHeader, intend bool) ([]byte, error) {
	return xmlEncode(data, GB2312, useHeader, lo.Ternary(intend, 2, 0))
}

func XmlDecode(data []byte, target any) error {
	decoder := XmlDecoder(data)
	return decoder.Decode(target)
}

func XmlUnmarshal[T any](data []byte) *T {
	var r T
	if err := XmlDecode(data, &r); err == nil {
		return &r
	}
	return nil
}

func XmlDecoder(data []byte) *xml.Decoder {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	return decoder
}
