package accrual

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
)

type wChans struct {
	quit      <-chan struct{}
	rateLimit chan<- time.Duration     // канал для отправки сигнала о превышении лимита запросов
	sleep     <-chan time.Duration     // канал для получения сигнала о начале режима ожидания
	toPost    <-chan orders.QueueOrder // очередь с ордерами для отправки
	toRepost  chan<- orders.QueueOrder // канал для
	toWrite   chan<- orders.QueueOrder // канал для передачи ордера для записи
	toCheck   <-chan orders.QueueOrder
}

type worker struct {
	id      int
	address string
	chs     wChans
	logger  *config.Logger
}

// run сейчас подумал, что наверное стоит разграничить воркеров на запись и проверку.
// Есть ли в этом смысл?
func (w *worker) run(wg *sync.WaitGroup) {
	w.logger.Debug().Msg(fmt.Sprintf("worker %d: running", w.id))
	wg.Add(1)
	defer wg.Done()
	for {
		select {
		case order := <-w.chs.toPost:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: got a post", w.id))
			w.post(order)
		case order := <-w.chs.toCheck:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: got a check", w.id))
			w.check(order)
		case timeout := <-w.chs.sleep:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: is sleeping", w.id))
			time.Sleep(time.Second * timeout)
		case <-w.chs.quit:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: closed", w.id))
			return
		}
	}
}

var postCount int64 = 0

// post обработка заказа на отправку
func (w *worker) post(order orders.QueueOrder) {
	if timeout, err := w.postRequest(order); err != nil {
		w.logger.Debug().Interface("err", err).Msg(fmt.Sprintf("worker %d: post func err", w.id))
		if errors.As(err, &TooManyRequestsError) {
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: too many request, sleep for 30s", w.id))
			w.chs.rateLimit <- timeout // если получили 429 отправляет сигнал в queue
			w.chs.toRepost <- order    // отправляет заказ обратно в очередь
		}
		return
	}
	order.Status = 2
	postCount += 1
	w.chs.toWrite <- order // после успесшной отправки заказа отдаем его под запись

	w.logger.Debug().Msg(fmt.Sprintf("worker %d: post done", w.id))
}

func getTimeout(str string) (time.Duration, error) {
	timeoutInt, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 60, err
	}

	return time.Duration(timeoutInt), nil
}

func (w *worker) postRequest(order orders.QueueOrder) (time.Duration, error) {
	s := fmt.Sprintf(
		`{ "order": "%d", "goods": [ { "description": "LG product", "price": 50000.0 } ] }`, order.Number,
	)

	reqBody := strings.NewReader(s)
	postAddress := fmt.Sprintf("%s/api/orders", w.address)
	w.logger.Debug().Msg(fmt.Sprintf("worker %d: posting %d", w.id, order.Number))
	req, err := http.Post(postAddress, "application/json", reqBody)
	if err != nil {
		return 0, err
	}

	defer req.Body.Close()

	switch {
	case req.StatusCode == http.StatusTooManyRequests:
		timeoutStr := req.Header.Get("Retry-After")
		timeout, err := getTimeout(timeoutStr)
		if err != nil {
			w.logger.Error().Caller().Err(err).Msg("can't parse timeout value")
		}
		return timeout, ThrowTooManyRequestsErr()
	case req.StatusCode == http.StatusInternalServerError:
		return 0, errors.New("500")
	default:
		return 0, nil
	}
}

var checkCount int64 = 0

func (w *worker) check(order orders.QueueOrder) {
	if timeout, err := w.checkRequest(&order); err != nil {
		if errors.As(err, &TooManyRequestsError) {
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: too many request, sleep for 30s", w.id))
			w.chs.rateLimit <- timeout
			w.chs.toWrite <- order
		}
		return
	}
	checkCount += 1
	w.chs.toWrite <- order

	w.logger.Debug().Msg(fmt.Sprintf("worker %d: check done", w.id))
}

type checkJSON struct {
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

func statusEncode(status string) int {
	table := map[string]int{
		"REGISTERED": 2,
		"PROCESSING": 2,
		"PROCESSED":  3,
		"INVALID":    4,
	}

	return table[status]
}

func (w *worker) checkRequest(order *orders.QueueOrder) (time.Duration, error) {
	getAddress := fmt.Sprintf("%s/api/orders/%d", w.address, order.Number)

	req, err := http.Get(getAddress)
	if err != nil {
		return 0, err
	}

	w.logger.Debug().Msg(fmt.Sprintf("check status code: %d", req.StatusCode))
	defer req.Body.Close()

	switch {
	case req.StatusCode == http.StatusTooManyRequests:
		timeoutStr := req.Header.Get("Retry-After")
		timeout, err := getTimeout(timeoutStr)
		if err != nil {
			w.logger.Error().Caller().Err(err).Msg("can't parse timeout value")
		}
		return timeout, ThrowTooManyRequestsErr()
	default:
		resBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return 0, err
		}

		data := checkJSON{}
		if err = json.Unmarshal(resBody, &data); err != nil {
			return 0, err
		}

		order.Status = statusEncode(data.Status)
		order.Accrual = data.Accrual
		return 0, nil
	}
}

func startWorker(id int, address string, chs wChans, logger *config.Logger) *worker {
	return &worker{
		id:      id + 1,
		address: address,
		chs:     chs,
		logger:  logger,
	}
}
