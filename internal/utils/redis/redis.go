package redis

import (
	"context"
	"fmt"
	"time"
	re "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"go.uber.org/zap"

	"irelia/pkg/logger/pkg"
	api "irelia/pkg/redis/api"
)

type Redis interface {
	Set(ctx context.Context, key string, value proto.Message, expireTime time.Duration) (bool, error)
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) (bool, error)
}

type redis struct {
	redis     *re.Client
	namespace string
}

func ReadConfig() *api.Redis {
	return &api.Redis{
		Address:   viper.GetString("redis.address"),
		Namespace: viper.GetString("redis.namespace"),
	}
}

func New(enable bool, cfg *api.Redis) Redis {
	if !enable {
		return Dummy()
	}

	client := re.NewClient(&re.Options{
        Addr: cfg.Address,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    if err := client.Ping(ctx).Err(); err != nil {
        logging.Logger(ctx).Error("Failed to connect to Redis", zap.String("address", cfg.Address), zap.Error(err))
    } else {
        logging.Logger(ctx).Info("Successfully connected to Redis")
    }

    return &redis{
        redis:     client,
        namespace: cfg.Namespace,
    }
}

func (r *redis) withNamespace(key string) string {
	return fmt.Sprintf("%s:%s", r.namespace, key)
}

func (r *redis) Set(ctx context.Context, key string, value proto.Message, expireTime time.Duration) (bool, error) {
	namespacedKey := r.withNamespace(key)
	jsonData, err := protojson.Marshal(value)
	if err != nil {
		return false, err
	}
	return r.redis.Set(ctx, namespacedKey, string(jsonData), expireTime).Err() == nil, nil
}

func (r *redis) Get(ctx context.Context, key string) ([]byte, error) {
	namespacedKey := r.withNamespace(key)
	val, err := r.redis.Get(ctx, namespacedKey).Result()
	if err == re.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return []byte(val), nil
}

func (r *redis) Delete(ctx context.Context, key string) (bool, error) {
	namespacedKey := r.withNamespace(key)
	result, err := r.redis.Del(ctx, namespacedKey).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}