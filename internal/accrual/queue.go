package accrual

import (
	"github.com/fedoroko/gophermart/internal/orders"
	"sync"
)

type Queue interface {
	Push(orders.QueueOrder)
	Pop() orders.QueueOrder
	Close() error
}

type queue struct {
	pool    []orders.QueueOrder
	poolMTX sync.Mutex
}

func (q *queue) Push(order orders.QueueOrder) {
	q.poolMTX.Lock()
	defer q.poolMTX.Unlock()

	q.pool = append(q.pool, order)
}

func (q *queue) Pop() (order *orders.QueueOrder) {
	q.poolMTX.Lock()
	defer q.poolMTX.Unlock()

	if len(q.pool) > 0 {
		order = &q.pool[0]
		q.pool = q.pool[1:]
	}

	return
}

func (q *queue) Close() error {
	return nil
}
