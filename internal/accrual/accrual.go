package accrual

import (
	"bytes"
	"context"
	"fmt"
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/fedoroko/gophermart/internal/storage"
	"net/http"
	"sync"
	"time"
)

var TooManyRequestsError *tooManyRequestsErr

type tooManyRequestsErr struct{}

func (e *tooManyRequestsErr) Error() string {
	return "invalid number"
}

func ThrowTooManyRequestsErr() *tooManyRequestsErr {
	return &tooManyRequestsErr{}
}

//go:generate mockgen -destination=../mocks/mock_doer_queue.go -package=mocks github.com/fedoroko/gophermart/internal/accrual Queue
type Queue interface {
	Push(*orders.Order) error
	Listen() error
	Close() error
}

type queue struct {
	quit      chan struct{}
	rateLimit chan struct{}
	sleep     chan struct{}
	poster    *poster
	checker   *checker
	workers   []*worker
	logger    *config.Logger
}

func (q *queue) Push(o *orders.Order) error {
	return q.poster.push(o)
}

func (q *queue) Listen() error {
	q.logger.Debug().Msg("accrual: LISTENING")
	var wg sync.WaitGroup
	for _, w := range q.workers {
		go w.run(&wg)
	}

	go q.poster.listen()
	go q.checker.listen()

	for {
		select {
		case <-q.quit:
			q.logger.Debug().Msg("accrual: CLOSED")
			return nil
		case <-q.rateLimit:
			for i := 1; i < len(q.workers); i++ {
				q.sleep <- struct{}{}
			}
		default:

		}

	}
}

func (q *queue) Close() error {
	close(q.quit)
	q.logger.Info().Msg("Queue closed")
	return nil
}

func (q *queue) setUpAccrual(address string) {
	address = fmt.Sprintf("http://%s/api/goods", address)
	body := []byte(`{ "match": "GO", "reward": 7, "reward_type": "%" }`)
	reqBody := bytes.NewBuffer(body)

	res, err := http.Post(address, "application/json", reqBody)
	if err != nil {
		q.logger.Error().Err(err).Send()
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		q.logger.Error().Msg(fmt.Sprintf("accrual setup failed, status code :%d", res.StatusCode))
	}
	q.logger.Info().Msg("accrual setup: SUCCESS")
}

type poster struct {
	quit     <-chan struct{}
	toPost   chan *orders.Order
	toRepost chan *orders.Order
	pool     []*orders.Order
	poolMTX  *sync.RWMutex
	logger   *config.Logger
}

func (p *poster) push(o *orders.Order) error {
	p.poolMTX.Lock()
	defer p.poolMTX.Unlock()
	p.pool = append(p.pool, o)
	p.logger.Debug().Msg("item pushed")
	return nil
}

func (p *poster) pop() *orders.Order {
	p.poolMTX.RLock()
	defer p.poolMTX.RUnlock()
	o := p.pool[0]
	p.pool = p.pool[1:]
	p.logger.Debug().Msg("item poped")
	return o
}

func (p *poster) listen() error {
	p.logger.Debug().Msg("poster: LISTENING")
	for {
		select {
		case <-p.quit:
			p.logger.Debug().Msg("poster: CLOSED")
			return nil
		default:
			if len(p.pool) > 0 {
				o := p.pop()
				p.toPost <- o
			}
		}
	}
}

type checker struct {
	quit       <-chan struct{}
	toWrite    chan *orders.Order
	toCheck    chan *orders.Order
	processing []*orders.Order
	processed  []*orders.Order
	db         storage.Repo
	logger     *config.Logger
}

func (c *checker) handleOrderStatus(o *orders.Order) {
	switch {
	case o.Status == 2:
		c.logger.Debug().Msg("appending to processing")
		c.processing = append(c.processing, o)
		if len(c.processing) == cap(c.processing) {
			c.flush()
		}
	default:
		c.logger.Debug().Msg("appending to processed")
		c.processed = append(c.processed, o)
		if len(c.processed) == cap(c.processed) {
			c.logger.Debug().Msg("flushing by cap")
			c.flush()
		}
	}
}

func (c *checker) listen() error {
	c.logger.Debug().Msg("checker: LISTENING")
	ticker := time.NewTicker(time.Second * 20)
	defer ticker.Stop()

	for {
		select {
		case o := <-c.toWrite:
			c.handleOrderStatus(o)
		case <-ticker.C:
			c.logger.Debug().Msg("flushing by ticker")
			c.flush()
		case <-c.quit:
			c.logger.Debug().Msg("flushing by quit")
			c.flush()
			c.logger.Debug().Msg("poster: CLOSED")
			return nil
		default:
			if len(c.processing) > 0 {
				o := c.processing[0]
				c.processing = c.processing[1:]
				c.toCheck <- o
			}
		}
	}
}

func (c *checker) flush() error {
	if len(c.processing) > 0 {
		c.logger.Debug().Msg("writing processing")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		err := c.db.OrdersUpdate(ctx, c.processing)
		if err != nil {
			c.logger.Error().Stack().Err(err).Msg("err on bd writing")
			return nil
		}
		c.processing = c.processing[:0]
	}
	if len(c.processed) > 0 {
		c.logger.Debug().Msg("writing processed")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		err := c.db.OrdersUpdate(ctx, c.processed)
		if err != nil {
			c.logger.Error().Stack().Err(err).Msg("err on bd writing")
			return nil
		}
		c.processed = c.processed[:0]
	}
	return nil
}

func newPoster(quit <-chan struct{}, logger *config.Logger) *poster {
	subLogger := logger.With().Str("Component", "Poster").Logger()
	return &poster{
		quit:     quit,
		toPost:   make(chan *orders.Order, 1000),
		toRepost: make(chan *orders.Order, 1000),
		pool:     []*orders.Order{},
		poolMTX:  &sync.RWMutex{},
		logger:   config.NewLogger(&subLogger),
	}
}

func newChecker(quit <-chan struct{}, db storage.Repo, logger *config.Logger) *checker {
	subLogger := logger.With().Str("Component", "Checker").Logger()
	return &checker{
		quit:       quit,
		toWrite:    make(chan *orders.Order, 1000),
		toCheck:    make(chan *orders.Order, 1000),
		processing: make([]*orders.Order, 0, 1000),
		processed:  make([]*orders.Order, 0, 1000),
		db:         db,
		logger:     config.NewLogger(&subLogger),
	}
}

func NewQueue(cfg *config.ServerConfig, db storage.Repo, logger *config.Logger) Queue {
	subLogger := logger.With().Str("Component", "Queue").Logger()

	quit := make(chan struct{})
	rateLimit := make(chan struct{})
	sleep := make(chan struct{})

	p := newPoster(quit, logger)
	c := newChecker(quit, db, logger)

	q := &queue{
		quit:      quit,
		rateLimit: rateLimit,
		sleep:     sleep,
		poster:    p,
		checker:   c,
		workers:   make([]*worker, cfg.WorkersCount),
		logger:    config.NewLogger(&subLogger),
	}

	//q.setUpAccrual(cfg.Accrual)
	chs := wChans{
		quit:      q.quit,
		rateLimit: q.rateLimit,
		sleep:     q.sleep,
		toPost:    p.toPost,
		toRepost:  p.toRepost,
		toWrite:   c.toWrite,
		toCheck:   c.toCheck,
	}

	for i := range q.workers {
		w := startWorker(i, cfg.Accrual, chs, q.logger)
		q.workers[i] = w
	}

	q.logger.Debug().Msg(fmt.Sprintf("%d workers prepared", cfg.WorkersCount))

	return q
}
