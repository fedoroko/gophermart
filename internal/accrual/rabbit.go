package accrual

import (
	"encoding/json"
	"github.com/streadway/amqp"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
)

type rabbitQueue struct {
	conn     *amqp.Connection
	ch       *amqp.Channel // нужны вместе с conn чтобы было что закрывать при завершении программы
	q        amqp.Queue    // в принципе не нужен, но на всякий случай
	outputCh chan<- orders.QueueOrder
}

func (r *rabbitQueue) Push(o orders.QueueOrder) error {
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
			ContentType: "application/json",
			Body:        data,
		})
}

func (r *rabbitQueue) Consume() error {
	msgs, err := r.ch.Consume(
		r.q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	for raw := range msgs {
		var order orders.QueueOrder
		err = json.Unmarshal(raw.Body, &order)
		if err != nil {

			continue
		}

		r.outputCh <- order
	}

	return nil
}

func (r *rabbitQueue) Close() error {
	if err := r.ch.Close(); err != nil {
		return err
	}

	return r.conn.Close()
}

func NewRabbitQueue(cfg *config.ServerConfig, name string, outputCh chan orders.QueueOrder) (Queue, error) {
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

	return &rabbitQueue{
		conn:     conn,
		ch:       ch,
		q:        q,
		outputCh: outputCh,
	}, nil
}
