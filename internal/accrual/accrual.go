package accrual

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/streadway/amqp"

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

	for _, o := range ors {
		q.Push(o)
	}
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
	quit        <-chan struct{}
	postQueue   rabbitQueue
	toRepost    <-chan *orders.Order
	repostQueue rabbitQueue
	logger      *config.Logger
}

func (p *poster) push(o *orders.Order) {
	if err := p.postQueue.publish(o); err != nil {
		p.logger.Error().Caller().Err(err).Send()
	}
}

func (p *poster) rePush(o *orders.Order) {
	if err := p.repostQueue.publish(o); err != nil {
		p.logger.Error().Caller().Err(err).Send()
	}
}

func (p *poster) listen() {
	for {
		select {
		case o := <-p.toRepost:
			p.rePush(o)
		case <-p.quit:
			if err := p.postQueue.close(); err != nil {
				p.logger.Error().Caller().Err(err).Send()
			}
			if err := p.repostQueue.close(); err != nil {
				p.logger.Error().Caller().Err(err).Send()
			}
			return
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
	checkQueue rabbitQueue
	processing []*orders.Order // пулл отправленных заказов
	processed  []*orders.Order // пулл проверенных заказов
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
			c.flush()
		case <-c.quit:
			c.flush()
			if err := c.checkQueue.close(); err != nil {
				c.logger.Error().Caller().Err(err).Send()
			}
			return nil
		default:
			if len(c.processing) > 0 {
				o := c.processing[0]
				c.processing = c.processing[1:]
				if err := c.checkQueue.publish(o); err != nil {
					c.logger.Error().Caller().Err(err).Send()
				}
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

type rabbitQueue struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	q    amqp.Queue
	msgs <-chan amqp.Delivery
}

func (r *rabbitQueue) publish(o *orders.Order) error {
	data, err := json.Marshal(o)
	if err != nil {
		return err
	}

	return r.ch.Publish(
		"",       // exchange
		r.q.Name, // routing key
		false,    // mandatory
		false,    // immediate
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
		name,  // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	return &rabbitQueue{
		conn: conn,
		ch:   ch,
		q:    q,
		msgs: msgs,
	}, nil
}

func newPoster(
	quit <-chan struct{}, toRepost chan *orders.Order, cfg *config.ServerConfig, logger *config.Logger) (*poster, error) {
	subLogger := logger.With().Str("Component", "Poster").Logger()

	postQueue, err := newRabbitQueue(cfg, "post")
	if err != nil {
		subLogger.Error().Caller().Err(err).Send()
		return nil, err
	}

	repostQueue, err := newRabbitQueue(cfg, "repost")
	if err != nil {
		subLogger.Error().Caller().Err(err).Send()
		return nil, err
	}

	return &poster{
		quit:        quit,
		postQueue:   *postQueue,
		toRepost:    toRepost,
		repostQueue: *repostQueue,
		logger:      config.NewLogger(&subLogger),
	}, err
}

func newChecker(
	quit <-chan struct{}, toWrite <-chan *orders.Order, cfg *config.ServerConfig,
	db storage.Repo, logger *config.Logger) (*checker, error) {
	subLogger := logger.With().Str("Component", "Checker").Logger()

	checkQueue, err := newRabbitQueue(cfg, "check")
	if err != nil {
		subLogger.Error().Caller().Err(err).Send()
		return nil, err
	}
	return &checker{
		quit:       quit,
		toWrite:    toWrite,
		checkQueue: *checkQueue,
		processing: make([]*orders.Order, 0, 1000),
		processed:  make([]*orders.Order, 0, 1000),
		db:         db,
		logger:     config.NewLogger(&subLogger),
	}, nil
}

func NewQueue(cfg *config.ServerConfig, db storage.Repo, logger *config.Logger) Queue {
	subLogger := logger.With().Str("Component", "Queue").Logger()

	quit := make(chan struct{})
	rateLimit := make(chan struct{})
	sleep := make(chan struct{})

	toRepost := make(chan *orders.Order, 1000)
	toWrite := make(chan *orders.Order, 1000)

	p, _ := newPoster(quit, toRepost, cfg, logger)
	c, _ := newChecker(quit, toWrite, cfg, db, logger)

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
		quit:        quit,
		rateLimit:   rateLimit,
		sleep:       sleep,
		postQueue:   p.postQueue.msgs,
		toRepost:    toRepost,
		repostQueue: p.repostQueue.msgs,
		toWrite:     toWrite,
		checkQueue:  c.checkQueue.msgs,
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
