package redisutil

import (
	"context"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/util/timeutil"
	"github.com/go-redis/redis/v8"
	"time"
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
        return {tostring(c), tostring(n), 1}
    end
    return {tostring(c), tostring(c), 0}
else
    redis.call('set', KEYS[1], n)
    return {'', tostring(n), 1}
end`
	zaddIf = `
local c = tonumber(redis.call('zscore', KEYS[1], ARGV[1]))
local n = tonumber(KEYS[2])
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

// NewScriptRedisClient 由于Go的依赖注入很麻烦，所以这里用的是显式挂载client
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

func parseResult(resp any) (string, string, bool) {
	r, ok := resp.([]interface{})
	if !ok || len(r) < 3 {
		log.Error("redis lua return type error")
	}
	return r[0].(string), r[1].(string), r[2].(int64) == 1
}

func (c *ScriptRedisClient) evalScript(scriptName string, keys []string, values ...any) (old, current string, ok bool) {
	ctx := context.Background()
	resp, err := c.client.EvalSha(ctx, c.scriptSha[scriptName], keys, values).Result()
	if err != nil {
		return
	}
	return parseResult(resp)
}

// SetIfHigher 如果value比原来的值大，则set
func (c *ScriptRedisClient) SetIfHigher(key string, value any) (old, current string, ok bool) {
	return c.evalScript("setIfHigher", []string{key}, value)
}

// SetIfLower 如果value比原来的值小，则set
func (c *ScriptRedisClient) SetIfLower(key string, value any) (old, current string, ok bool) {
	return c.evalScript("setIfLower", []string{key}, value)
}

// SetIfNotEqual 如果value和原来的值不相等，则set
func (c *ScriptRedisClient) SetIfNotEqual(key string, value any) (old, current string, ok bool) {
	return c.evalScript("setIfNotEqual", []string{key}, value)
}

// ZaddIfHigher 如果item的score比原来的大，则set
func (c *ScriptRedisClient) ZaddIfHigher(key, item string, score int64) (old, current string, ok bool) {
	return c.evalScript("zaddIfHigher", []string{key, fmt.Sprint(score)}, item)
}

// ZaddIfLower 如果item的score比原来的小，则set
func (c *ScriptRedisClient) ZaddIfLower(key, item string, score int64) (old, current string, ok bool) {
	return c.evalScript("zaddIfLower", []string{key, fmt.Sprint(score)}, item)
}

// ZaddIfNotEqual 如果item的score和原来不相等，则set
func (c *ScriptRedisClient) ZaddIfNotEqual(key, item string, score int64) (old, current string, ok bool) {
	return c.evalScript("zaddIfNotEqual", []string{key, fmt.Sprint(score)}, item)
}

// NeedRateLimit 是否需要限流
// count: 次数，period：时间（单位秒）
func (c *ScriptRedisClient) NeedRateLimit(key string, count int, period int64) (bool, error) {
	ctx := context.Background()
	ts := timeutil.GetNowTs()
	resp, err := c.client.EvalSha(ctx, c.scriptSha["slideWindowRateLimit"],
		[]string{key}, ts, ts-period, period).Result()
	if err != nil {
		return false, err
	}
	return resp.(int64) > int64(count), nil
}

// DelIfEqual 相等则删除key
func (c *ScriptRedisClient) DelIfEqual(key string, value any) (bool, error) {
	ctx := context.Background()
	resp, err := c.client.EvalSha(ctx, c.scriptSha["delIfEqual"],
		[]string{key}, value).Result()
	if err != nil {
		return false, err
	}
	return resp.(int64) == 1, nil
}

// HDelIfEqual 相等则哈希删除key
func (c *ScriptRedisClient) HDelIfEqual(key, hashKey string, value any) (bool, error) {
	ctx := context.Background()
	resp, err := c.client.EvalSha(ctx, c.scriptSha["hdelIfEqual"],
		[]string{key}, hashKey, value).Result()
	if err != nil {
		return false, err
	}
	return resp.(int64) == 1, nil
}

// HGetSet GetSet的哈希版本，返回旧值
func (c *ScriptRedisClient) HGetSet(key, hashKey string, value any) (string, error) {
	ctx := context.Background()
	resp, err := c.client.EvalSha(ctx, c.scriptSha["hgetset"],
		[]string{key}, hashKey, value).Result()
	if err != nil {
		return "", err
	}
	return resp.(string), nil
}

// TryLock 单机的分布式锁
func (c *ScriptRedisClient) TryLock(key string, mark any, period int64) (bool, error) {
	ctx := context.Background()
	return c.client.SetNX(ctx, key, mark, time.Duration(period)*time.Second).Result()
}

// UnLock 解除锁定，仅mark与现有值相等时才删除
func (c *ScriptRedisClient) UnLock(key string, mark any) (bool, error) {
	return c.DelIfEqual(key, mark)
}
