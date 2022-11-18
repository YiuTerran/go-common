package transport

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/sip"
	"net"
)

// TODO migrate other factories to functional arguments

type Options struct {
	MessageMapper sip.MessageMapper
	Fields        log.Fields
}

type LayerOption interface {
	ApplyLayer(opts *LayerOptions)
}

type LayerOptions struct {
	Options
	DNSResolver *net.Resolver
}

type ProtocolOption interface {
	ApplyProtocol(opts *ProtocolOptions)
}

type ProtocolOptions struct {
	Options
}

func WithMessageMapper(mapper sip.MessageMapper) interface {
	LayerOption
	ProtocolOption
} {
	return withMessageMapper{mapper}
}

type withMessageMapper struct {
	mapper sip.MessageMapper
}

func (o withMessageMapper) ApplyLayer(opts *LayerOptions) {
	opts.MessageMapper = o.mapper
}

func (o withMessageMapper) ApplyProtocol(opts *ProtocolOptions) {
	opts.MessageMapper = o.mapper
}

func WithFields(fields log.Fields) interface {
	LayerOption
	ProtocolOption
} {
	return withFields{fields}
}

type withFields struct {
	fields log.Fields
}

func (o withFields) ApplyLayer(opts *LayerOptions) {
	opts.Fields = o.fields
}

func (o withFields) ApplyProtocol(opts *ProtocolOptions) {
	opts.Fields = o.fields
}

func WithDNSResolver(resolver *net.Resolver) LayerOption {
	return withDnsResolver{resolver}
}

type withDnsResolver struct {
	resolver *net.Resolver
}

func (o withDnsResolver) ApplyLayer(opts *LayerOptions) {
	opts.DNSResolver = o.resolver
}

// ListenOption show Listen method options
type ListenOption interface {
	ApplyListen(opts *ListenOptions)
}

type ListenOptions struct {
	TLSConfig TLSConfig
	Mock      bool
}

type ListenMode struct {
	Mock bool
}

func (l ListenMode) ApplyListen(opts *ListenOptions) {
	opts.Mock = l.Mock
}
