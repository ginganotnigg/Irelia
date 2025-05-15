package redis

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
)

type dummy struct {
	Redis
}

func Dummy() Redis {
	return &dummy{}
}

func (d *dummy) Set(ctx context.Context, key string, value proto.Message, expireTime time.Duration) (bool, error) {
	return false, nil
}
func (d *dummy) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}

func (d *dummy) Delete(ctx context.Context, key string) (bool, error) {
	return false, nil
}