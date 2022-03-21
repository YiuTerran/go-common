package redis

import (
	"context"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/go-redis/redis/v8"
)

/**	 一些常用的redis功能
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
        return {c, n, 1}
    end
    return {c, c, 0}
else
    redis.call('set', KEYS[1], n)
    return {nil, n, 1}
end`
	zaddIf = `
local c = tonumber(redis.call('zscore', KEYS[1], ARGV[1]))
local n = tonumber(KEYS[2])
if c then
		%s      
		redis.call('zadd', KEYS[1], n, ARGV[1])
        return {c, n, 1}
    end
    return {c, c, 0}
else
    redis.call('zadd', KEYS[1], n, ARGV[1])
    return {nil, n, 1}
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
	hgetset = `redis.call('hget',KEYS[1],ARGV[1]); redis.call('hset',KEYS[1],ARGV[1],ARGV[2]); return old;`
)

var (
	scriptNameMap = map[string]string{
		"sefIfHigher":          fmt.Sprintf(setIf, "if n > c then"),
		"sefIfLower":           fmt.Sprintf(setIf, "if n < c then"),
		"setIfNotEqual":        fmt.Sprintf(setIf, "if n ~= c then"),
		"zaddIfHigher":         fmt.Sprintf(zaddIf, "if n > c then"),
		"zaddIfLower":          fmt.Sprintf(zaddIf, "if n < c then"),
		"zaddIfNotEqual":       fmt.Sprintf(zaddIf, "if n ~= c then"),
		"slideWindowRateLimit": slideWindowRateLimit,
		"delIfEqual":           delIfEqual,
		"hdelIfEqual":          hdelIfEqual,
		"hgetset":              hgetset,
	}
)

type ScriptRedisClient struct {
	client    *redis.Client
	scriptSha map[string]string
}

func NewScriptRedisClient(client *redis.Client) *ScriptRedisClient {
	ctx := context.Background()
	src := &ScriptRedisClient{client: client, scriptSha: make(map[string]string, len(scriptNameMap))}
	for name, script := range scriptNameMap {
		sha, err := client.ScriptLoad(ctx, script).Result()
		if err != nil {
			log.Error("fail to load script %s", name)
			continue
		}
		src.scriptSha[name] = sha
	}
	return src
}

func parseResult(resp any) (int64, int64, bool) {
	r, ok := resp.([]interface{})
	if !ok || len(r) < 3 {
		log.Error("redis lua return type error")
	}
	return r[0].(int64), r[1].(int64), r[2].(int64) == 1
}

func (c *ScriptRedisClient) evalScript(scriptName string, keys []string, values ...any) (old, current int64, ok bool) {
	ctx := context.Background()
	resp, err := c.client.EvalSha(ctx, c.scriptSha["name"], keys, values).Result()
	if err != nil {
		return
	}
	return parseResult(resp)
}

func (c *ScriptRedisClient) SetIfHigher(key string, value int64) (old, current int64, ok bool) {
	return c.evalScript("setIfHigher", []string{key}, value)
}

func (c *ScriptRedisClient) SetIfLower(key string, value int64) (old, current int64, ok bool) {
	return c.evalScript("setIfLower", []string{key}, value)
}

func (c *ScriptRedisClient) SetIfNotEqual(key string, value int64) (old, current int64, ok bool) {
	return c.evalScript("setIfNotEqual", []string{key}, value)
}

func (c *ScriptRedisClient) zaddIfHigher(key, item string, score int64) (old, current int64, ok bool) {
	return c.evalScript("zaddIfHigher", []string{key, fmt.Sprint(score)}, item)
}

func (c *ScriptRedisClient) zaddIfLower(key, item string, score int64) (old, current int64, ok bool) {
	return c.evalScript("zaddIfLower", []string{key, fmt.Sprint(score)}, item)
}

func (c *ScriptRedisClient) zaddIfNotEqual(key, item string, score int64) (old, current int64, ok bool) {

}