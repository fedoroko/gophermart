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

type tooManyRequestsErr struct {
}

func (e *tooManyRequestsErr) Error() string {
	return "invalid number"
}

func ThrowTooManyRequestsErr() *tooManyRequestsErr {
	return &tooManyRequestsErr{}
}

//go:generate mockgen -destination=../mocks/mock_doer_workerpool.go -package=mocks github.com/fedoroko/gophermart/internal/accrual WorkerPool
type WorkerPool interface {
	Push(*orders.Order) error
	Listen() error
	Close() error
}

type workerPool struct {
	quit      chan struct{}        // канал завершения программы
	rateLimit <-chan time.Duration // канал с сигналом от воркера о превышении лимита запросов
	sleep     chan<- time.Duration // канал с сигналом для воркеров, о начале режима ожидания
	poster    *poster              // посылает заказы в accrual
	checker   *checker             // проверяет заказы из accrual
	workers   []*worker
	logger    *config.Logger
}

// Push enqueue order
// new order workflow:
// handler -> controller -> db -> controller -> queue -> db
func (p *workerPool) Push(order *orders.Order) error {
	p.poster.push(order.ToQueue())
	return nil
}

func (p *workerPool) Listen() error {
	p.logger.Debug().Msg("accrual: LISTENING")

	var wg sync.WaitGroup
	for _, w := range p.workers {
		go w.run(&wg)
	}

	go p.poster.listen()
	go p.checker.listen()

	for {
		select {
		case <-p.quit:
			p.logger.Debug().Msg("accrual: CLOSED")
			return nil
		case timeout := <-p.rateLimit:
			// в случае получения сигнала о превышении лимита запросов от первого из воркеров
			// дает команду остальным (n - 1) воркерам перейти в режим ожидания
			for i := 1; i < len(p.workers); i++ {
				p.sleep <- timeout
			}
		}
	}
}

func (p *workerPool) Close() error {
	close(p.quit)
	p.logger.Info().Msg("Queue closed")
	return nil
}

func (p *workerPool) setUpAccrual(address string) {
	address = fmt.Sprintf("%s/api/goods", address)
	body := []byte(`{ "match": "LG", "reward": 7, "reward_type": "%" }`)
	reqBody := bytes.NewBuffer(body)

	res, err := http.Post(address, "application/json", reqBody)
	if err != nil {
		p.logger.Error().Err(err).Send()
		return
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		p.logger.Warn().Msg(fmt.Sprintf("accrual setup failed, status code :%d", res.StatusCode))
	}
	p.logger.Info().Msg("accrual setup: SUCCESS")
}

// restore adding orders from db with statuses 1 and 2 (NEW, PROCESSING) to queue
// for some fallback cases
func (p *workerPool) restore() error {
	ors, err := p.checker.db.OrdersRestore(context.Background())
	if err != nil {
		if errors.As(err, &orders.NoItemsError) {
			return nil
		}
		return err
	}

	p.logger.Debug().Msg("restoring queue")

	for _, order := range ors {
		p.Push(order)
	}
	p.logger.Debug().Msg("queue restored")

	return nil
}

// poster отвечает за отправку ордеров в accrual
// из queue.Push ордер попадает в postQueue.
// Из postQueue заказы обрабатывают воркеры
// если при отправке воркер получит ошибку (например 429), в таком случае
// воркер отправляет заказ обратно постеру, где он снова возвращается в очередь,
// но с повышенным приоритетом
type poster struct {
	quit     <-chan struct{}
	queue    Queue
	toPost   chan<- orders.QueueOrder
	toRepost <-chan orders.QueueOrder
	logger   *config.Logger
}

func (p *poster) push(order orders.QueueOrder) {
	if err := p.queue.Push(order); err != nil {
		p.logger.Error().Caller().Err(err).Send()
	}
}

func (p *poster) listen() {
	go p.queue.Consume()
	for {
		select {
		case order := <-p.toRepost:
			p.push(order)
		case <-p.quit:
			if err := p.queue.Close(); err != nil {
				p.logger.Error().Caller().Err(err).Send()
			}
			return
		}
	}
}

// checker отвечает за проверку результатов обработки заказов
// и запись в базу, как заказов еще не отправленных на проверку, так и уже проверенных.
// После отправки заказа worker отпраляет его в toWrite, где его подбирает checker
// и вносит в pool, а оттуда отправляет в checkQueue, где его получит воркер.
// После результатов проверки воркер снова возвращает заказ в toWrite.
// В случае неудачной проверки отправлять заказ в отдельную очередь можно,
// но на мой взгляд избыточно. Важно как можно скорее зафиксировать отправку,
// а проверку можно и подождать.
// Размышляю над заменой слайса для пулла на мапу с ключом по айди, тогда,
// в большинстве случаев, будет одна запись в бд вместо двух.
type checker struct {
	quit    <-chan struct{}
	toWrite <-chan orders.QueueOrder // канал ордерами под запись
	queue   Queue
	pool    []orders.QueueOrder // пулл заказов
	db      storage.Repo
	logger  *config.Logger
}

// handleOrderStatus в зависимости от статуса перенаправляет заказ из toWrite
// в checkQueue или просто записывает.
func (c *checker) handleOrderStatus(o orders.QueueOrder) {
	if o.Status == 2 {
		if err := c.queue.Push(o); err != nil {
			c.logger.Error().Caller().Err(err).Send()
		}
	}

	c.pool = append(c.pool, o)
	if len(c.pool) == cap(c.pool) {
		c.flush()
	}
}

// listen слушает каналы и осуществляет запись в бд.
// Для экономии ресурсов, записываем батчами. Запись происходит по наступлению
// лимита пулла или по таймеру.
func (c *checker) listen() error {
	c.logger.Debug().Msg("checker: LISTENING")
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()

	go c.queue.Consume()

	for {
		select {
		case o := <-c.toWrite:
			c.handleOrderStatus(o)
		case <-ticker.C:
			c.flush()
		case <-c.quit:
			c.flush()
			if err := c.queue.Close(); err != nil {
				c.logger.Error().Caller().Err(err).Send()
			}
			return nil
		}
	}
}

// flush записывает ордера из pool в базу.
func (c *checker) flush() error {
	if len(c.pool) == 0 {
		return nil
	}
	c.logger.Debug().Msg("writing pool")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	err := c.db.OrdersUpdate(ctx, c.pool)
	if err != nil {
		c.logger.Error().Caller().Stack().Err(err).Msg("err on bd writing")
		for i := 0; i < 3; i++ {
			c.logger.Warn().Msg("Can't connect to bd. Sleep 30s")
			time.Sleep(time.Second * 30)
			err = c.db.OrdersUpdate(context.Background(), c.pool)
			if err == nil {
				break
			}
		}

		return err
	}

	c.logger.Debug().Msg("writing success")
	c.pool = c.pool[:0]

	return nil
}

func newPoster(
	quit <-chan struct{}, toPost chan orders.QueueOrder, toRepost chan orders.QueueOrder,
	cfg *config.ServerConfig, logger *config.Logger) (*poster, error) {
	subLogger := logger.With().Str("Component", "Poster").Logger()

	var postQueue Queue
	if cfg.RabbitMQ != "" {
		pq, err := NewRabbitQueue(cfg, "post", toPost)
		if err != nil {
			subLogger.Error().Caller().Err(err).Send()
			return nil, err
		}
		postQueue = pq
	} else {
		postQueue = NewQueue(toPost)
	}

	return &poster{
		quit:     quit,
		queue:    postQueue,
		toPost:   toPost,
		toRepost: toRepost,
		logger:   config.NewLogger(&subLogger),
	}, nil
}

func newChecker(
	quit <-chan struct{}, toWrite chan orders.QueueOrder, toCheck chan orders.QueueOrder, cfg *config.ServerConfig,
	db storage.Repo, logger *config.Logger) (*checker, error) {
	subLogger := logger.With().Str("Component", "Checker").Logger()

	var checkQueue Queue
	if cfg.RabbitMQ != "" {
		cq, err := NewRabbitQueue(cfg, "check", toCheck)
		if err != nil {
			subLogger.Error().Caller().Err(err).Send()
			return nil, err
		}
		checkQueue = cq
	} else {
		checkQueue = NewQueue(toCheck)
	}

	return &checker{
		quit:    quit,
		toWrite: toWrite,
		queue:   checkQueue,
		pool:    make([]orders.QueueOrder, 0, 1000),
		db:      db,
		logger:  config.NewLogger(&subLogger),
	}, nil
}

func NewWorkerPool(cfg *config.ServerConfig, db storage.Repo, logger *config.Logger) WorkerPool {
	subLogger := logger.With().Str("Component", "Queue").Logger()

	quit := make(chan struct{})
	rateLimit := make(chan time.Duration)
	sleep := make(chan time.Duration)

	toPost := make(chan orders.QueueOrder, 1000)
	toRepost := make(chan orders.QueueOrder, 1000)

	p, _ := newPoster(quit, toPost, toRepost, cfg, logger)

	toWrite := make(chan orders.QueueOrder, 1000)
	toCheck := make(chan orders.QueueOrder, 1000)

	c, _ := newChecker(quit, toWrite, toCheck, cfg, db, logger)

	q := &workerPool{
		quit:      quit,
		rateLimit: rateLimit,
		sleep:     sleep,
		poster:    p,
		checker:   c,
		workers:   make([]*worker, cfg.WorkersCount),
		logger:    config.NewLogger(&subLogger),
	}

	q.setUpAccrual(cfg.Accrual) // с настройкой не проходит тесты
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
