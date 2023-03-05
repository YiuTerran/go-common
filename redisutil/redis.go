package redisutil

import (
	"fmt"
	"github.com/go-redis/redis/v8"
)

/**	 一些常用的redis lua脚本
  *  @author tryao
  *  @date 2022/03/21 15:21
**/

const (
	setIf = `
local c = tonumber(redis.call('get', KEYS[1]))
local n = tonumber(ARGV[1])
if c then 
		%s
		redis.call('set', KEYS[1], n)
        return {tostring(c), tostring(n), 1}
    end
    return {tostring(c), tostring(c), 0}
else
    redis.call('set', KEYS[1], n)
    return {'', tostring(n), 1}
end`
	zaddIf = `
local c = tonumber(redis.call('zscore', KEYS[1], ARGV[1]))
local n = tonumber(ARGV[2])
if c then
		%s      
		redis.call('zadd', KEYS[1], n, ARGV[1])
        return {tostring(c), tostring(n), 1}
    end
    return {tostring(c), tostring(c), 0}
else
    redis.call('zadd', KEYS[1], n, ARGV[1])
    return {'', tostring(n), 1}
end`
	slideWindowRateLimit = `
redis.call('ZADD', KEYS[1], tonumber(ARGV[1]), ARGV[1])
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, tonumber(ARGV[2]))
local c = redis.call('ZCARD', KEYS[1])
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[3]))
return c;`
	delIfEqual = `
local c = redis.call('get', KEYS[1])
if c and c == ARGV[1] then
    redis.call('del', KEYS[1])
    return 1
else
    return 0
end`
	hdelIfEqual = `
local c = redis.call('hget', KEYS[1], ARGV[1])
if c and c == ARGV[2] then
    redis.call('hdel', KEYS[1], ARGV[1])
    return 1
else
    return 0
end`
	hsetIfKeyExist = `
if redis.call('exists', KEYS[1]) == 1 then
    redis.call('hset', KEYS[1], ARGV[1], ARGV[2])
    return 1
end
return 0
`
	hmsetIfKeyExist = `
if redis.call('exists', KEYS[1]) == 1 then
    redis.call('hmset', KEYS[1], unpack(ARGV))
    return 1
end
return 0
`
	hgetset    = `redis.call('hget',KEYS[1],ARGV[1]); redis.call('hset',KEYS[1],ARGV[1],ARGV[2]); return old;`
	ttlIfEqual = `
local c = redis.call('get',KEYS[1])
if c and c == ARGV[1] then
	redis.call('expire', KEYS[1], ARGV[2])
    return 1
elseif not c then
	redis.call('set', KEYS[1], ARGV[1], 'ex', ARGV[2])
	return 1
end
return 0
`
)

var (
	SetIfHigher          = redis.NewScript(fmt.Sprintf(setIf, "if n > c then"))
	SetIfLower           = redis.NewScript(fmt.Sprintf(setIf, "if n < c then"))
	SetIfNotEqual        = redis.NewScript(fmt.Sprintf(setIf, "if n ~= c then"))
	ZAddIfHigher         = redis.NewScript(fmt.Sprintf(zaddIf, "if n > c then"))
	ZAddIfLower          = redis.NewScript(fmt.Sprintf(zaddIf, "if n < c then"))
	ZAddIfNotEqual       = redis.NewScript(fmt.Sprintf(zaddIf, "if n ~= c then"))
	SlideWindowRateLimit = redis.NewScript(slideWindowRateLimit)
	DelIfEqual           = redis.NewScript(delIfEqual)
	HDelIfEqual          = redis.NewScript(hdelIfEqual)
	HGetSet              = redis.NewScript(hgetset)
	HSetIfKeyExist       = redis.NewScript(hsetIfKeyExist)
	HMSetIfKeyExist      = redis.NewScript(hmsetIfKeyExist)
	ExpireIfEqual        = redis.NewScript(ttlIfEqual)
)
