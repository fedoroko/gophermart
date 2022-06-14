package controllers

import (
	"context"
	"io"

	"github.com/fedoroko/gophermart/internal/accrual"
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/fedoroko/gophermart/internal/storage"
	"github.com/fedoroko/gophermart/internal/users"
	"github.com/fedoroko/gophermart/internal/withdrawals"
)

type Controller interface {
	Login(context.Context, io.Reader) (string, error)
	Register(context.Context, io.Reader) (string, error)
	Logout(context.Context, *users.Session) error

	Order(context.Context, *users.User, io.Reader) error
	Orders(context.Context, *users.User) ([]*orders.Order, error)

	Withdraw(context.Context, *users.User, io.Reader) error
	Withdrawals(context.Context, *users.User) ([]*withdrawals.Withdrawal, error)
}

type controller struct {
	r      storage.Repo
	q      accrual.Queue
	logger *config.Logger
}

func Ctrl(r storage.Repo, q accrual.Queue, logger *config.Logger) Controller {
	subLogger := logger.With().Str("Component", "Controller").Logger()
	return &controller{
		r:      r,
		q:      q,
		logger: config.NewLogger(&subLogger),
	}
}

func (c *controller) Login(ctx context.Context, body io.Reader) (string, error) {
	user, err := users.FromJSON(body)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("failed to parse users data")
		return "", err
	}

	session, err := c.r.UserExists(ctx, user)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("db conflict")
		return "", err
	}

	return session.Token(), nil
}

func (c *controller) Register(ctx context.Context, body io.Reader) (string, error) {
	user, err := users.FromJSON(body)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("failed to parse users data")
		return "", err
	}

	session, err := c.r.UserCreate(ctx, user)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("db conflict")
		return "", err
	}

	return session.Token(), nil
}

func (c *controller) Logout(ctx context.Context, s *users.Session) error {
	return nil
}

func (c *controller) Order(ctx context.Context, u *users.User, body io.Reader) error {
	o, err := orders.FromPlain(u, body)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("failed to parse order")
		return err
	}

	err = c.r.OrderCreate(ctx, o)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("db conflict")
		return err
	}

	if err = c.q.Push(o); err != nil {
		c.logger.Debug().Caller().Err(err).Msg("failed to enqueue")
		return err
	}

	return nil
}

func (c *controller) Orders(ctx context.Context, u *users.User) ([]*orders.Order, error) {
	ors, err := c.r.UserOrders(ctx, u)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("db conflict")
		return nil, err
	}

	return ors, nil
}

func (c *controller) Withdraw(ctx context.Context, u *users.User, body io.Reader) error {
	w, err := withdrawals.FromJSON(u, body)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("failed to parse withdrawals data")
		return err
	}

	err = c.r.WithdrawalCreate(ctx, w)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("db conflict")
		return err
	}

	return nil
}

func (c *controller) Withdrawals(ctx context.Context, u *users.User) ([]*withdrawals.Withdrawal, error) {
	ws, err := c.r.UserWithdrawals(ctx, u)
	if err != nil {
		c.logger.Debug().Caller().Err(err).Msg("db conflict")
		return nil, err
	}

	return ws, nil
}
