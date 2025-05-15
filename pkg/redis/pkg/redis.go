package redis

import (
	"context"
	"time"

	redis "github.com/redis/go-redis/v9"
	redistrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/redis/go-redis.v9"

	api "irelia/pkg/redis/api"
)

func New(config *api.Redis, opts ...Option) (*redis.Client, error) {
	o := &Opt{
		Options: &redis.Options{
			Addr: config.GetAddress(),
		},
	}
	if len(config.GetUsername()) > 0 {
		o.Username = config.GetUsername()
	}
	if len(config.GetPassword()) > 0 {
		o.Password = config.GetPassword()
	}
	if config.GetDb() > 0 {
		o.DB = int(config.GetDb())
	}
	if config.GetMaxRetries() != 0 {
		o.MaxRetries = int(config.GetMaxRetries())
	}
	if config.GetMinRetryBackoff() != 0 {
		o.MinRetryBackoff = time.Duration(config.GetMinRetryBackoff()) * time.Millisecond
	}
	if config.GetMaxRetryBackoff() != 0 {
		o.MaxRetryBackoff = time.Duration(config.GetMaxRetryBackoff()) * time.Millisecond
	}
	if config.GetDialTimeout() != 0 {
		o.DialTimeout = time.Duration(config.GetDialTimeout()) * time.Millisecond
	}
	if config.GetReadTimeout() != 0 {
		o.ReadTimeout = time.Duration(config.GetReadTimeout()) * time.Millisecond
	}
	if config.GetWriteTimeout() != 0 {
		o.WriteTimeout = time.Duration(config.GetWriteTimeout()) * time.Millisecond
	}
	if config.GetPoolFifo() {
		o.PoolFIFO = config.GetPoolFifo()
	}
	if config.GetPoolSize() != 0 {
		o.PoolSize = int(config.GetPoolSize())
	}
	if config.GetPoolTimeout() != 0 {
		o.PoolTimeout = time.Duration(config.GetPoolTimeout()) * time.Millisecond
	}
	if config.GetMinIdleConns() != 0 {
		o.MinIdleConns = int(config.GetMinIdleConns())
	}
	if config.GetTls() != nil && config.GetTls().GetEnabled() {
		tlsConfig, err := NewTLS(config.GetTls())
		if err != nil {
			return nil, err
		}
		o.TLSConfig = tlsConfig
	}
	if len(config.GetClientName()) > 0 {
		o.ClientName = config.GetClientName()
	}

	for _, o0 := range opts {
		o0.Apply(o)
	}

	client := redis.NewClient(o.Options)
	client.AddHook(&nsHook{config.GetNamespace()})
	client.AddHook(&debugHook{config.GetDebug()})

	redistrace.WrapClient(client, redistrace.WithServiceName("redis"))
	return client, client.Ping(context.Background()).Err()
}

type Opt struct {
	*redis.Options
}

type Option interface {
	Apply(o *Opt)
}

type OptionFunc func(*Opt)

func (f OptionFunc) Apply(o *Opt) {
	f(o)
}

// Limiter interface used to implemented circuit breaker or rate limiter.
func Limiter(limiter redis.Limiter) Option {
	return OptionFunc(func(o *Opt) {
		o.Limiter = limiter
	})
}