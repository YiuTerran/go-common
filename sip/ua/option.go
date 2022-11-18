package ua

import (
	"github.com/YiuTerran/go-common/sip/auth"
	"github.com/YiuTerran/go-common/sip/sip"
)

/**
  *  @author tryao
  *  @date 2022/03/31 14:05
**/

type RequestWithContextOption interface {
	ApplyRequestWithContext(options *RequestWithContextOptions)
}

type RequestWithContextOptions struct {
	ResponseHandler func(res sip.Response, request sip.Request)
	Authorizer      auth.Authorizer
}

type withResponseHandler struct {
	handler func(res sip.Response, request sip.Request)
}

func (o withResponseHandler) ApplyRequestWithContext(options *RequestWithContextOptions) {
	options.ResponseHandler = o.handler
}

func WithResponseHandler(handler func(res sip.Response, request sip.Request)) RequestWithContextOption {
	return withResponseHandler{handler}
}

type withAuthorizer struct {
	authorizer auth.Authorizer
}

func (o withAuthorizer) ApplyRequestWithContext(options *RequestWithContextOptions) {
	options.Authorizer = o.authorizer
}

func WithAuthorizer(authorizer auth.Authorizer) RequestWithContextOption {
	return withAuthorizer{authorizer}
}
