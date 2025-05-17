package rabbit

import (
	"context"
	"fmt"
	"irelia/pkg/logger/pkg"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
	
	rb "irelia/pkg/rabbit/api"
)

type Rabbit interface {
	Consume(ctx context.Context, consumeFunction func(ctx context.Context, msg amqp.Delivery) error) error
	Publish(ctx context.Context, body []byte) error
}

type rabbit struct {
	connectionUrl string
	comsumeQueue  string
	publicQueue   string
	maxConsumer   int32
	expireTime    int32
}

func ReadConfig() *rb.RabbitMQ {
	return &rb.RabbitMQ{
		Address:      viper.GetString("rabbitmq.address"),
		Port:         viper.GetInt32("rabbitmq.port"),
		Username:     viper.GetString("rabbitmq.username"),
		Password:     viper.GetString("rabbitmq.password"),
		ConsumeQueue: viper.GetString("rabbitmq.consume_queue"),
		PublicQueue:  viper.GetString("rabbitmq.public_queue"),
		MaxConsumer:  viper.GetInt32("rabbitmq.max_consumer"),
		ExpireTime:   viper.GetInt32("rabbitmq.expire_time"),
	}
}

func New(rb *rb.RabbitMQ) Rabbit {
	if rb == nil {
		return &Dummy{}
	}

	connectionUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", rb.Username, rb.Password, rb.Address, rb.Port)
	return &rabbit{
		connectionUrl: connectionUrl,
		comsumeQueue:  rb.ConsumeQueue,
		publicQueue:   rb.PublicQueue,
		maxConsumer:   rb.MaxConsumer,
		expireTime:    rb.ExpireTime,
	}
}

func (r *rabbit) processMessage(ctx context.Context, msg amqp.Delivery, sem chan struct{}, consumeFunction func(ctx context.Context, msg amqp.Delivery) error) {
	logging.Logger(ctx).Info(fmt.Sprintf("Received: %s", msg.Body))
	defer func() { <-sem }()

	if err := consumeFunction(ctx, msg); err != nil {
		logging.Logger(ctx).Error(fmt.Sprintf("Error: %s", err.Error()))
		msg.Nack(false, true)
	} else {
		logging.Logger(ctx).Info(fmt.Sprintf("Acknowledge: %s", msg.Body))
		msg.Ack(false)
	}
}

func (r *rabbit) Consume(ctx context.Context, consumeFunction func(ctx context.Context, msg amqp.Delivery) error) error {

	conn, err := amqp.Dial(r.connectionUrl)
	if err != nil {
		return err
	}
	defer conn.Close()

	logging.Logger(ctx).Info("Connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(r.comsumeQueue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	sem := make(chan struct{}, r.maxConsumer)

	for msg := range msgs {
		sem <- struct{}{}
		go r.processMessage(ctx, msg, sem, consumeFunction)
	}

	return nil
}

func (r *rabbit) Publish(ctx context.Context, body []byte) error {
	conn, err := amqp.Dial(r.connectionUrl)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(r.publicQueue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	err = ch.Publish("", q.Name, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
		Expiration:  fmt.Sprintf("%d", r.expireTime),
	})
	if err != nil {
		return err
	}

	logging.Logger(ctx).Info(fmt.Sprintf("Sent: %s", string(body)))
	return nil
}
