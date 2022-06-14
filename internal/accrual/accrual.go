package accrual

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/fedoroko/gophermart/internal/storage"
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
	quit      chan struct{}   // канал завершения программы
	rateLimit <-chan struct{} // канал с сигналом от воркера о превышении лимита запросов
	sleep     chan<- struct{} // канал с сигналом для воркеров, о начале режима ожидания
	poster    *poster         // посылает заказы в accrual
	checker   *checker        // проверяет заказы из accrual
	workers   []*worker
	logger    *config.Logger
}

// Push enqueue order
// new orders workflow:
// handler -> controller -> db -> controller -> queue
func (q *queue) Push(o *orders.Order) error {
	q.poster.push(o)
	return nil
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
			// в случае получения сигнала о превышении лимита запросов от первого из воркеров
			// дает команду остальным (n - 1) воркерам перейти в режим ожидания
			for i := 1; i < len(q.workers); i++ {
				q.sleep <- struct{}{}
			}
		}
	}
}

func (q *queue) Close() error {
	close(q.quit)
	q.logger.Info().Msg("Queue closed")
	return nil
}

func (q *queue) setUpAccrual(address string) {
	address = fmt.Sprintf("%s/api/goods", address)
	body := []byte(`{ "match": "LG", "reward": 7, "reward_type": "%" }`)
	reqBody := bytes.NewBuffer(body)

	res, err := http.Post(address, "application/json", reqBody)
	if err != nil {
		q.logger.Error().Err(err).Send()
		return
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		q.logger.Warn().Msg(fmt.Sprintf("accrual setup failed, status code :%d", res.StatusCode))
	}
	q.logger.Info().Msg("accrual setup: SUCCESS")
}

// restore adding orders from db with statuses 1 and 2 (NEW, PROCESSING) to queue
// for some fallback cases
func (q *queue) restore() error {
	ors, err := q.checker.db.OrdersRestore(context.Background())
	if err != nil {
		if errors.As(err, &orders.NoItemsError) {
			return nil
		}
		return err
	}

	q.logger.Debug().Msg("restoring queue")

	q.poster.poolMTX.Lock()
	defer q.poster.poolMTX.Unlock()

	q.poster.pool = append(q.poster.pool, ors...)
	q.logger.Debug().Msg("queue restored")

	return nil
}

// poster отвечает за отправку ордеров в accrual
// из queue.Push ордер попадает в pool
// если отправлять напрямую в канал, при высокой нагрузке возможны зависания
// из pool заказы уходят в канал toPost, где их обрабатывают воркеры
// если при отправке воркер получит ошибку (например 429), в таком случае
// воркер отправляет заказ обратно постеру, где он снова возвращается в очередь,
// но с повышенным приоритетом
type poster struct {
	quit      <-chan struct{}
	toPost    chan<- *orders.Order // канал с новыми ордерами
	toRepost  <-chan *orders.Order // канал с неудачными ордерами
	pool      []*orders.Order      // хранилище новых ордеров
	poolMTX   *sync.RWMutex
	rePool    []*orders.Order // хранилище неудачных ордеров
	rePoolMTX *sync.RWMutex
	logger    *config.Logger
}

func (p *poster) push(o *orders.Order) {
	p.poolMTX.Lock()
	defer p.poolMTX.Unlock()

	p.pool = append(p.pool, o)
	p.logger.Debug().Msg("item pushed")
}

func (p *poster) rePush(o *orders.Order) {
	p.rePoolMTX.Lock()
	defer p.rePoolMTX.Unlock()

	p.rePool = append(p.rePool, o)
	p.logger.Debug().Msg("item repushed")
}

func (p *poster) pop() *orders.Order {
	if len(p.rePool) > 0 {
		p.rePoolMTX.Lock()
		defer p.rePoolMTX.Unlock()

		o := p.rePool[0]
		p.rePool = p.rePool[1:]
		p.logger.Debug().Msg("item repoped")
		return o
	}
	if len(p.pool) > 0 {
		p.poolMTX.RLock()
		defer p.poolMTX.RUnlock()

		o := p.pool[0]
		p.pool = p.pool[1:]
		p.logger.Debug().Msg("item poped")
		return o
	}

	return nil
}

func (p *poster) listen() error {
	p.logger.Debug().Msg("poster: LISTENING")
	for {
		select {
		case o := <-p.toRepost:
			p.rePush(o)
		case <-p.quit:
			p.logger.Debug().Msg("poster: CLOSED")
			return nil
		default:
			o := p.pop()
			if o != nil {
				p.toPost <- o
			}
		}
	}
}

// checker отвечает за проверку результатов обработки заказов
// и запись в базу, как заказов еще не отправленных на проверку, так и уже проверенных.
// После отправки заказа worker отпраляет его в toWrite, где его подбирает checker
// и вносит в processing, а оттуда отправляет в канал toCheck, где его получит воркер.
// После результатов проверки воркер снова возвращает заказ в toWrite, но на этот раз
// заказ попадает в processed и будет удален из цепочки, после записи в базу.
// В случае неудачной проверки отправлять заказ в отдельную очередь можно,
// но на мой взгляд избыточно. Важно как можно скорее зафиксировать отправку,
// а проверку можно и подождать.
type checker struct {
	quit       <-chan struct{}
	toWrite    <-chan *orders.Order // канал с проверенным оредрами от воркеров
	toCheck    chan<- *orders.Order // канал с оредрами, которые необходимо проверить
	processing []*orders.Order      // пулл отправленных заказов
	processed  []*orders.Order      // пулл проверенных заказов
	db         storage.Repo
	logger     *config.Logger
}

// handleOrderStatus в зависимости от статуса перенаправляет заказ из toWrite
// в processing или processed
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

// listen слушает каналы, раскладывает ордера по processing и processed
// и осуществляет запись в бд.
// Для экономии ресурсов, записываем батчами. Запись происходит по наступлению
// лимита каждого из пуллов или по таймеру. При маленькой нагрузке, высока вероятность,
// что до истечения таймера заказ успеет отправиться и провериться, в таком случае
// будет всего одна запись вместо двух. Все ордера получаем из каналов, поэтому мутекс не нужен.
func (c *checker) listen() error {
	c.logger.Debug().Msg("checker: LISTENING")
	ticker := time.NewTicker(time.Second * 2)
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

// flush записывает ордера из processing и processed в базу.
func (c *checker) flush() error {
	if len(c.processing) > 0 {
		c.logger.Debug().Msg("writing processing")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		err := c.db.OrdersUpdate(ctx, c.processing)
		if err != nil {
			c.logger.Error().Caller().Stack().Err(err).Msg("err on bd writing")
			return nil
		}
	}
	if len(c.processed) > 0 {
		c.logger.Debug().Msg("writing processed")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		err := c.db.OrdersUpdate(ctx, c.processed)
		if err != nil {
			c.logger.Error().Caller().Stack().Err(err).Msg("err on bd writing")
			return nil
		}
		c.processed = c.processed[:0] // очистка processed после записи
	}
	return nil
}

func newPoster(
	quit <-chan struct{}, toPost chan<- *orders.Order, toRepost <-chan *orders.Order, logger *config.Logger) *poster {
	subLogger := logger.With().Str("Component", "Poster").Logger()
	return &poster{
		quit:      quit,
		toPost:    toPost,
		toRepost:  toRepost,
		pool:      []*orders.Order{},
		poolMTX:   &sync.RWMutex{},
		rePool:    []*orders.Order{},
		rePoolMTX: &sync.RWMutex{},
		logger:    config.NewLogger(&subLogger),
	}
}

func newChecker(
	quit <-chan struct{}, toWrite <-chan *orders.Order, toCheck chan<- *orders.Order, db storage.Repo, logger *config.Logger) *checker {
	subLogger := logger.With().Str("Component", "Checker").Logger()
	return &checker{
		quit:       quit,
		toWrite:    toWrite,
		toCheck:    toCheck,
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
	toPost := make(chan *orders.Order, 1000)
	toRepost := make(chan *orders.Order, 1000)
	toWrite := make(chan *orders.Order, 1000)
	toCheck := make(chan *orders.Order, 1000)

	p := newPoster(quit, toPost, toRepost, logger)
	c := newChecker(quit, toWrite, toCheck, db, logger)

	q := &queue{
		quit:      quit,
		rateLimit: rateLimit,
		sleep:     sleep,
		poster:    p,
		checker:   c,
		workers:   make([]*worker, cfg.WorkersCount),
		logger:    config.NewLogger(&subLogger),
	}

	//q.setUpAccrual(cfg.Accrual) // с настройкой не проходит тесты
	chs := wChans{
		quit:      quit,
		rateLimit: rateLimit,
		sleep:     sleep,
		toPost:    toPost,
		toRepost:  toRepost,
		toWrite:   toWrite,
		toCheck:   toCheck,
	}

	for i := range q.workers {
		w := startWorker(i, cfg.Accrual, chs, q.logger)
		q.workers[i] = w
	}

	q.logger.Debug().Msg(fmt.Sprintf("%d workers prepared", cfg.WorkersCount))

	if err := q.restore(); err != nil {
		q.logger.Error().Caller().Err(err).Send()
	}
	return q
}
