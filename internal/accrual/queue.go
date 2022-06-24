package accrual

import (
	"github.com/fedoroko/gophermart/internal/orders"
)

type Queue interface {
	Push(orders.QueueOrder) error
	Consume() error
	Close() error
}

type queue struct {
	outputCh chan<- orders.QueueOrder
}

func (q *queue) Push(order orders.QueueOrder) error {
	q.outputCh <- order
	return nil
}

func (q *queue) Consume() error {
	return nil
}

func (q *queue) Close() error {
	return nil
}

func NewQueue(outputCh chan orders.QueueOrder) Queue {
	return &queue{
		outputCh: outputCh,
	}
}
