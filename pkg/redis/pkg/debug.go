package redis

import (
	"context"

	redis "github.com/redis/go-redis/v9"

	"irelia/pkg/logger/pkg"
)

type debugHook struct {
	enabled bool
}

func (h *debugHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *debugHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.enabled {
			logging.Logger(ctx).Debug(cmd.String())
		}

		return next(ctx, cmd)
	}
}

func (h *debugHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmd []redis.Cmder) error {
		if h.enabled {
			for _, c := range cmd {
				logging.Logger(ctx).Debug(c.String())
			}
		}

		return next(ctx, cmd)
	}
}