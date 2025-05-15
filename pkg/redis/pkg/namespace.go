package redis

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	redis "github.com/redis/go-redis/v9"
)

type nsHook struct {
	namespace string
}

func (h *nsHook) appendNamespace(key interface{}) string {
	k := fmt.Sprint(key)
	if strings.HasPrefix(k, h.namespace+":") {
		return k
	}

	return fmt.Sprintf("%s:%s", h.namespace, k)
}

func (h *nsHook) updateCmd(cmd redis.Cmder) {
	if len(cmd.Args()) <= 1 {
		return
	}

	switch cmd.Name() {
	case "dump", "expire", "expireat", "move", "persist", "pexpire", "pexpireat",
		"pttl", "restore", "sort", "ttl", "type", "append", "decr", "decrby", "get",
		"getrange", "getset", "getex", "getdel", "incr", "incrby", "incrbyfloat",
		"set", "setex", "setnx", "setrange", "strlen", "getbit", "setbit", "bitcount",
		"bitpos", "bitfield", "sscan", "hscan", "zscan", "hdel", "hexists", "hget",
		"hgetall", "hincrby", "hincrbyfloat", "hkeys", "hlen", "hmget", "hset", "hmset",
		"hsetnx", "hvals", "hrandfield", "lindex", "linsert", "llen", "lpop", "lpos",
		"lpush", "lpushx", "lrange", "lrem", "lset", "ltrim", "rpop", "rpush", "rpushx",
		"sadd", "scard", "sismember", "smismember", "smembers", "spop", "srandmember",
		"srem", "xtrim", "zadd", "zcard", "zcount", "zlexcount", "zincrby", "zmscore",
		"zpopmax", "zpopmin", "zrange", "zrangebyscore", "zrangebylex", "zrank", "zrem",
		"zremrangebyrank", "zremrangebyscore", "zremrangebylex", "zrevrange", "zrevrangebyscore",
		"zrevrangebylex", "zrevrank", "zscore", "zrandmember", "pfadd", "geoadd", "georadius_ro",
		"georadius", "georadiusbymember_ro", "georadiusbymember", "geosearch", "geodist",
		"geohash", "geopos":
		cmd.Args()[1] = h.appendNamespace(cmd.Args()[1])
	case "del", "unlink", "exists", "rename", "renamenx", "touch", "mget", "sdiff",
		"sinter", "sunion", "pfcount", "pfmerge":
		for i := 1; i < len(cmd.Args()); i++ {
			cmd.Args()[i] = h.appendNamespace(cmd.Args()[i])
		}
	case "migrate":
		cmd.Args()[3] = h.appendNamespace(cmd.Args()[3])
	case "object", "xinfo", "geosearchstore":
		cmd.Args()[2] = h.appendNamespace(cmd.Args()[2])
	case "zrangestore":
		cmd.Args()[1] = h.appendNamespace(cmd.Args()[1])
		cmd.Args()[2] = h.appendNamespace(cmd.Args()[2])
	case "mset", "msetnx":
		for i := 1; i < len(cmd.Args()); i++ {
			if i%2 == 0 {
				continue
			}
			cmd.Args()[i] = h.appendNamespace(cmd.Args()[i])
		}
	case "bitop", "sdiffstore", "sinterstore", "sunionstore", "zdiff":
		for i := 2; i < len(cmd.Args()); i++ {
			cmd.Args()[i] = h.appendNamespace(cmd.Args()[i])
		}
	case "blpop", "brpop", "bzpopmax", "bzpopmin":
		for i := 1; i < len(cmd.Args())-1; i++ {
			cmd.Args()[i] = h.appendNamespace(cmd.Args()[i])
		}
	case "zinterstore", "zunionstore", "zdiffstore":
		cmd.Args()[1] = h.appendNamespace(cmd.Args()[1])
		numKeys := cmd.Args()[2].(int)
		if numKeys > 0 {
			for i := 3; i < numKeys+3; i++ {
				cmd.Args()[i] = h.appendNamespace(cmd.Args()[i])
			}
		}
	case "zinter", "zunion":
		numKeys := cmd.Args()[1].(int)
		if numKeys > 0 {
			for i := 2; i < numKeys+2; i++ {
				cmd.Args()[i] = h.appendNamespace(cmd.Args()[i])
			}
		}
	case "eval", "evalsha":
		numKeys := cmd.Args()[2].(int)
		if numKeys > 0 {
			for i := 3; i < numKeys+3; i++ {
				cmd.Args()[i] = h.appendNamespace(cmd.Args()[i])
			}
		}
	case "keys":
		pattern := cmd.Args()[1].(string)
		if unicode.IsLetter([]rune(pattern)[0]) || unicode.IsNumber([]rune(pattern)[0]) {
			cmd.Args()[1] = h.appendNamespace(cmd.Args()[1])
		}
	case "scan":
		var matchIndex = -1
		for i, arg := range cmd.Args() {
			if fmt.Sprint(arg) == "match" {
				matchIndex = i
				break
			}
		}
		if matchIndex >= 0 && matchIndex+1 < len(cmd.Args()) {
			cmd.Args()[matchIndex+1] = h.appendNamespace(cmd.Args()[matchIndex+1])
		}
	}

	switch cmd.FullName() {
	case "cluster keyslot":
		cmd.Args()[2] = h.appendNamespace(cmd.Args()[2])
	}
}

func (h *nsHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *nsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if len(h.namespace) > 0 {
			h.updateCmd(cmd)
		}

		return next(ctx, cmd)
	}
}

func (h *nsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmd []redis.Cmder) error {
		if len(h.namespace) > 0 {
			for _, c := range cmd {
				h.updateCmd(c)
			}
		}

		return next(ctx, cmd)
	}
}