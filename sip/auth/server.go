package auth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"github.com/samber/lo"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/util/timeutil"
	"github.com/YiuTerran/go-common/sip/sip"
	"math/rand"
	"strings"
	"sync"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/04/06 09:35
**/

const (
	NonceExpire = 3 * time.Minute
)

type Session struct {
	nonce      string
	expireTime time.Time
}

// CheckResult 业务层校验结果
type CheckResult struct {
	User     any
	Username string
	Password string
	Status   sip.StatusCode
}

// UacParam UAC认证需要的参数
type UacParam struct {
	Algorithm  string
	Realm      string
	UseAuthInt bool
	Status     sip.StatusCode
}

type Config struct {
	// CheckRegister Register请求验证
	CheckRegister func(ctx context.Context, req sip.Request, authArgs sip.Params) CheckResult
	// AuthParam 自定义认证参数
	AuthParam func(ctx context.Context, req sip.Request) UacParam
	// CheckUser 非Register请求校验是否已认证过，返回用户信息或者需要认证的参数
	CheckUser func(ctx context.Context, req sip.Request) (user any, param UacParam)
	// CloseSig 相关线程销毁信号
	CloseSig chan struct{}
}

// ServerAuthorizer Proxy-Authorization | WWW-Authenticate
type ServerAuthorizer struct {
	// a map[call id]authSession pair
	sessions map[string]Session
	*Config

	mx sync.RWMutex
}

// NewServerAuthorizer 一般一个服务可以共用同一个认证器，注意这里都是同步调用，要防止阻塞的话可以异步调用Authenticate
func NewServerAuthorizer(config *Config) *ServerAuthorizer {
	auth := &ServerAuthorizer{
		sessions: make(map[string]Session),
		Config:   config,
	}
	timeutil.Schedule(func() {
		auth.mx.Lock()
		for k, v := range auth.sessions {
			if time.Now().After(v.expireTime) {
				delete(auth.sessions, k)
				log.Debug("callId %s auth expired", k)
			}
		}
		auth.mx.Unlock()
	}, 5*time.Second, config.CloseSig)
	return auth
}

// Authenticate handles server auth requests.
// user:用户相关信息，这个数据实际上还是来自业务自己
// isNew: 是否重新认证，虽然标准中没写，实测在运行中如果返回401，设备可能并不会重新Register，而是直接在下一个请求中带上Authorization字段
func (auth *ServerAuthorizer) Authenticate(ctx context.Context, request sip.Request, tx sip.ServerTransaction) (user any, isNew bool) {
	from, _ := request.From()
	headers := request.GetHeaders("Authorization")

	if request.Method() == sip.REGISTER && len(headers) == 0 {
		// 提示用户认证
		auth.requestAuthentication(ctx, request, tx, from)
		return nil, false
	} else if len(headers) > 0 {
		authenticateHeader := headers[0].(*sip.GenericHeader)
		authArgs := parseAuthHeader(authenticateHeader.Contents)
		return auth.checkAuthorization(ctx, request, tx, authArgs, from), true
	} else {
		// 非Register请求，确认请求者已经注册过
		user, param := auth.CheckUser(ctx, request)
		if user == nil {
			log.Debug("req user not authed:%s", request.String())
			auth.requestAuthentication(ctx, request, tx, from, param)
			return nil, false
		}
		return user, false
	}
}

func (auth *ServerAuthorizer) requestAuthentication(ctx context.Context, request sip.Request, tx sip.ServerTransaction,
	from *sip.FromHeader, params ...UacParam) {
	callID, ok := request.CallID()
	if !ok {
		sendResponse(request, tx, 400, "Missing required Call-ID header.")
		return
	}
	response := sip.NewResponseFromRequest(request.MessageID(), request, 401, "Unauthorized", "")
	nonce := geneGrateNonce(8)
	opaque := geneGrateNonce(4)
	// 避免重复计算，某些时候这个值应该是已经知道的
	var param UacParam
	if len(params) == 0 {
		param = auth.AuthParam(ctx, request)
	} else {
		param = params[0]
	}
	if param.Status != 0 {
		sendResponse(request, tx, param.Status, "")
		return
	}
	digest := sip.NewParams(sip.AuthParams)
	digest.Add("realm", sip.String{Str: sip.QuotedString(param.Realm)})
	if param.UseAuthInt {
		digest.Add("qop", sip.String{Str: `"auth,auth-int"`})
	} else {
		digest.Add("qop", sip.String{Str: `"auth"`})
	}
	digest.Add("nonce", sip.String{Str: sip.QuotedString(nonce)})
	digest.Add("opaque", sip.String{Str: sip.QuotedString(opaque)})
	digest.Add("stale", sip.String{Str: "FALSE"})
	digest.Add("algorithm", sip.String{Str: "MD5"})

	response.AppendHeader(&sip.GenericHeader{
		HeaderName: "WWW-Authenticate",
		Contents:   "Digest " + digest.String(),
	})

	from.Params.Add("tag", sip.String{Str: geneGrateNonce(8)})
	// 3分钟之内需要认证完毕
	auth.mx.Lock()
	auth.sessions[callID.String()] = Session{
		nonce:      nonce,
		expireTime: time.Now().Add(NonceExpire),
	}
	auth.mx.Unlock()
	response.SetBody("", true)
	if err := tx.Respond(response); err != nil {
		log.Warn("fail to send need-auth resp, err:%s", err)
	}
}

func (auth *ServerAuthorizer) checkAuthorization(ctx context.Context, request sip.Request, tx sip.ServerTransaction,
	authArgs sip.Params, from *sip.FromHeader) any {
	callID, ok := request.CallID()
	if !ok {
		sendResponse(request, tx, 400, "Missing required Call-ID header.")
		return nil
	}

	auth.mx.RLock()
	session, found := auth.sessions[callID.String()]
	auth.mx.RUnlock()
	if !found {
		auth.requestAuthentication(ctx, request, tx, from)
		return nil
	}

	if time.Now().After(session.expireTime) {
		auth.requestAuthentication(ctx, request, tx, from)
		return nil
	}

	//不应该要求username与from中的user相同

	if nonce, ok := authArgs.Get("nonce"); ok && nonce.String() != session.nonce {
		auth.requestAuthentication(ctx, request, tx, from)
		return nil
	}

	r := auth.CheckRegister(ctx, request, authArgs)
	if r.Status != 0 {
		tips := lo.Ternary(r.Status >= 500, "Server Internal Error", "Auth Error")
		sendResponse(request, tx, r.Status, tips)
		return nil
	}
	// 如果密码为空，视为不需要认证
	if r.Password == "" {
		return r.User
	}
	uri, _ := authArgs.Get("uri")
	nc, _ := authArgs.Get("nc")
	cnonce, _ := authArgs.Get("cnonce")
	response, _ := authArgs.Get("response")
	qop, _ := authArgs.Get("qop")
	realm, _ := authArgs.Get("realm")
	if uri == nil {
		sendResponse(request, tx, sip.StatusForbidden, "Forbidden (Bad auth)")
	}
	result := ""

	// HA1 = MD5(A1) = MD5(username:realm:password).
	ha1 := md5Hex(r.Username + ":" + realm.String() + ":" + r.Password)
	if qop != nil && qop.String() == "auth" && cnonce != nil && nc != nil {
		// HA2 = MD5(A2) = MD5(method:digestURI).
		ha2 := md5Hex(string(request.Method()) + ":" + uri.String())

		// Response = MD5(HA1:nonce:nonceCount:credentialsNonce:qop:HA2).
		result = md5Hex(ha1 + ":" + session.nonce + ":" + nc.String() +
			":" + cnonce.String() + ":auth:" + ha2)
	} else if qop != nil && qop.String() == "auth-int" && cnonce != nil && nc != nil {
		// HA2 = MD5(A2) = MD5(method:digestURI:MD5(entityBody)).
		ha2 := md5Hex(string(request.Method()) + ":" + uri.String() + ":" + md5Hex(request.Body()))

		// Response = MD5(HA1:nonce:nonceCount:credentialsNonce:qop:HA2).
		result = md5Hex(ha1 + ":" + session.nonce + ":" + nc.String() +
			":" + cnonce.String() + ":auth-int:" + ha2)
	} else {
		// HA2 = MD5(A2) = MD5(method:digestURI).
		ha2 := md5Hex(string(request.Method()) + ":" + uri.String())

		// Response = MD5(HA1:nonce:HA2).
		result = md5Hex(ha1 + ":" + session.nonce + ":" + ha2)
	}

	if result != response.String() {
		sendResponse(request, tx, sip.StatusForbidden, "Forbidden (Bad auth)")
		return nil
	}
	return r.User
}

// parseAuthHeader .
func parseAuthHeader(value string) sip.Params {
	authArgs := sip.NewParams(sip.AuthParams)
	matches := authPattern.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		authArgs.Add(match[1], sip.String{Str: strings.Replace(match[2], "\"", "", -1)})
	}
	return authArgs
}

func geneGrateNonce(size int) string {
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func md5Hex(data string) string {
	sum := md5.Sum([]byte(data))
	return hex.EncodeToString(sum[:])
}

// sendResponse .
func sendResponse(request sip.Request, tx sip.ServerTransaction, statusCode sip.StatusCode, reason string) {
	response := sip.NewResponseFromRequest(request.MessageID(), request, statusCode, reason, "")
	if err := tx.Respond(response); err != nil {
		log.Warn("fail to send resp %s, err:%s", response.String(), err)
	}
}
