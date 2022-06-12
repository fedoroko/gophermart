package controllers

import (
	"context"
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/fedoroko/gophermart/internal/storage"
	"github.com/fedoroko/gophermart/internal/users"
	"io"
)

type Controller interface {
	Login(context.Context, io.Reader) (string, error)
	Register(context.Context, io.Reader) (string, error)
	Logout(context.Context, *users.Session) error
	Order(context.Context, *users.User, io.Reader) error
}

type controller struct {
	r      storage.Repo
	logger *config.Logger
}

func Ctrl(r storage.Repo, logger *config.Logger) Controller {
	subLogger := logger.With().Str("Component", "Controller").Logger()
	return &controller{
		r:      r,
		logger: config.NewLogger(&subLogger),
	}
}

func (c *controller) Login(ctx context.Context, body io.Reader) (string, error) {
	user, err := users.FromJSON(body)
	if err != nil {
		return "", err
	}

	session, err := c.r.UserExists(ctx, user)
	if err != nil {
		return "", err
	}

	return session.Token(), nil
}

func (c *controller) Register(ctx context.Context, body io.Reader) (string, error) {
	user, err := users.FromJSON(body)
	if err != nil {
		return "", err
	}

	session, err := c.r.UserCreate(ctx, user)
	if err != nil {
		return "", err
	}

	return session.Token(), nil
}

func (c *controller) Logout(ctx context.Context, s *users.Session) error {
	return nil
}

func (c *controller) Order(ctx context.Context, u *users.User, body io.Reader) error {
	temp, err := orders.FromPlain(u, body, false)
	if err != nil {
		return err
	}

	order, err := c.r.OrderCreate(ctx, temp)
	if err != nil {
		return err
	}

	if err = order.Process(); err != nil {
		return err
	}

	return nil
}

func (c *controller) Orders(ctx context.Context, u *users.User) ([]*orders.Order, error) {
	ors, err := c.r.UserOrders(ctx, u)
	if err != nil {
		return nil, err
	}

	return ors, nil
}
