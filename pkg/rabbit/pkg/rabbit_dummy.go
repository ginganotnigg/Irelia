package rabbit

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Dummy struct{}

func (n *Dummy) Consume(ctx context.Context, consumeFunction func(ctx context.Context, msg amqp.Delivery) error) error {
	return nil
}

func (n *Dummy) Publish(ctx context.Context, body []byte) error {
	return nil
}