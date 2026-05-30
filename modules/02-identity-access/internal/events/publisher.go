package events

import amqp "github.com/streadway/amqp"

// AMQPInterface wraps amqp.Connection and amqp.Channel operations
// so the Publisher is testable without a live RabbitMQ instance.
type AMQPInterface interface {
	Close() error
	QueueDeclare(
		name string,
		durable bool,
		autoDelete bool,
		exclusive bool,
		noWait bool,
		args amqp.Table,
	) (amqp.Queue, error)
	Publish(
		exchange string,
		key string,
		mandatory bool,
		immediate bool,
		msg amqp.Publishing,
	) error
}
