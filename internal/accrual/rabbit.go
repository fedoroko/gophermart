package accrual

import (
	"encoding/json"

	"github.com/streadway/amqp"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
)

type rabbitQueue struct {
	conn *amqp.Connection
	ch   *amqp.Channel        // нужны вместе с conn чтобы было что закрывать при завершении программы
	q    amqp.Queue           // в принципе не нужен, но на всякий случай
	msgs <-chan amqp.Delivery // канал с сообщениями из очереди
}

func (r *rabbitQueue) publish(o orders.QueueOrder) error {
	data, err := json.Marshal(o)
	if err != nil {
		return err
	}

	return r.ch.Publish(
		"",
		r.q.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        data,
		})
}

func (r *rabbitQueue) close() error {
	if err := r.ch.Close(); err != nil {
		return err
	}

	return r.conn.Close()
}

func newRabbitQueue(cfg *config.ServerConfig, name string) (*rabbitQueue, error) {
	conn, err := amqp.Dial(cfg.RabbitMQ)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	q, err := ch.QueueDeclare(
		name,
		false,
		false,
		false,
		false,
		nil,
	)

	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)

	return &rabbitQueue{
		conn: conn,
		ch:   ch,
		q:    q,
		msgs: msgs,
	}, nil
}
