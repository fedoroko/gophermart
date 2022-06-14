package accrual

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

type wChans struct {
	quit      <-chan struct{}
	rateLimit chan<- struct{}
	sleep     <-chan struct{}
	toPost    <-chan *orders.Order
	toRepost  chan *orders.Order
	toWrite   chan<- *orders.Order
	toCheck   chan *orders.Order
}

type worker struct {
	id      int
	address string
	chs     wChans
	logger  *config.Logger
}

func (w *worker) run(wg *sync.WaitGroup) {
	w.logger.Debug().Msg(fmt.Sprintf("worker %d: running", w.id))
	wg.Add(1)
	defer wg.Done()
	for {
		select {
		case o := <-w.chs.toRepost:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: got a requeued post", w.id))
			w.post(o)
		case o := <-w.chs.toPost:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: got a post", w.id))
			w.post(o)
		case o := <-w.chs.toCheck:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: got a check", w.id))
			w.check(o)
		case <-w.chs.sleep:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: is sleeping", w.id))
			time.Sleep(time.Second * 30)
		case <-w.chs.quit:
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: closed", w.id))
			return
		}
	}
}

func (w *worker) post(o *orders.Order) {
	w.logger.Debug().Msg(fmt.Sprintf("worker %d: post func", w.id))
	if err := w.postRequest(o); err != nil {
		w.logger.Debug().Interface("err", err).Msg(fmt.Sprintf("worker %d: post func err", w.id))
		if errors.As(err, &TooManyRequestsError) {
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: too many request, sleep for 30s", w.id))
			w.chs.rateLimit <- struct{}{}
			w.chs.toRepost <- o
			time.Sleep(time.Second * 30)
		}
		return
	}
	w.chs.toWrite <- o

	w.logger.Debug().Msg(fmt.Sprintf("worker %d: post done", w.id))
}

type data struct {
	Order int64 `json:"order"`
	Goods struct {
		Description string  `json:"description"`
		Price       float64 `json:"price"`
	}
}

func (w *worker) postRequest(o *orders.Order) error {
	s := fmt.Sprintf(
		`{ "order": "%d", "goods": [ { "description": "LG product", "price": 50000.0 } ] }`, o.Number,
	)

	reqBody := strings.NewReader(s)
	postAddress := fmt.Sprintf("%s/api/orders", w.address)

	req, err := http.Post(postAddress, "application/json", reqBody)
	w.logger.Debug().Interface("err", err).Int("code", req.StatusCode).Send()
	if err != nil {
		return err
	}

	defer req.Body.Close()

	switch {
	case req.StatusCode == http.StatusTooManyRequests:
		return ThrowTooManyRequestsErr()
	case req.StatusCode == http.StatusAccepted:
		o.Status = 2
		return nil
	default:
		o.Status = 4
		return nil
	}
}

func (w *worker) check(o *orders.Order) {
	if err := w.checkRequest(o); err != nil {
		if errors.As(err, &TooManyRequestsError) {
			w.logger.Debug().Msg(fmt.Sprintf("worker %d: too many request, sleep for 30s", w.id))
			w.chs.rateLimit <- struct{}{}
			w.chs.toCheck <- o
			time.Sleep(time.Second * 30)
		}
		return
	}
	w.chs.toWrite <- o

	w.logger.Debug().Msg(fmt.Sprintf("worker %d: check done", w.id))
}

type checkJSON struct {
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

func statusEncode(status string) int {
	table := map[string]int{
		"REGISTERED": 1,
		"PROCESSING": 2,
		"PROCESSED":  3,
		"INVALID":    4,
	}

	return table[status]
}

func (w *worker) checkRequest(o *orders.Order) error {
	getAddress := fmt.Sprintf("%s/api/orders/%d", w.address, o.Number)

	req, err := http.Get(getAddress)
	w.logger.Debug().Interface("err", err).Int("code", req.StatusCode).Send()
	if err != nil {
		return err
	}

	w.logger.Debug().Msg(fmt.Sprintf("check status code: %d", req.StatusCode))
	defer req.Body.Close()
	switch {
	case req.StatusCode == http.StatusTooManyRequests:
		return ThrowTooManyRequestsErr()
	default:
		resBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}

		data := checkJSON{}
		if err = json.Unmarshal(resBody, &data); err != nil {
			return err
		}

		o.Status = statusEncode(data.Status)
		o.Accrual = data.Accrual

		return nil
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
