package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fedoroko/gophermart/internal/accrual"
	"github.com/fedoroko/gophermart/internal/withdrawals"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/controllers"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/fedoroko/gophermart/internal/storage"
	"github.com/fedoroko/gophermart/internal/users"
)

type handler struct {
	ctrl    controllers.Controller
	logger  *config.Logger
	timeout time.Duration
}

func Handler(r storage.Repo, q accrual.Queue, logger *config.Logger, timeout time.Duration) *handler {
	ctrl := controllers.Ctrl(r, q, logger)
	subLogger := logger.With().Str("Component", "Handler").Logger()
	return &handler{
		ctrl:    ctrl,
		logger:  config.NewLogger(&subLogger),
		timeout: timeout,
	}
}

func (h *handler) LoginFunc(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	token, err := h.ctrl.Login(ctx, c.Request.Body)
	if err != nil {
		switch {
		case errors.As(err, &users.WrongPairError):
			c.AbortWithStatusJSON(http.StatusUnauthorized, err.Error())
		case errors.As(err, &users.BadFormatError):
			c.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
		default:
			h.logger.Error().Stack().Err(err).Send()
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	c.Header("Authorization", token)
	c.JSON(http.StatusOK, token)
}

func (h *handler) RegisterFunc(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	token, err := h.ctrl.Register(ctx, c.Request.Body)
	if err != nil {
		switch {
		case errors.As(err, &users.AlreadyExistsError):
			c.AbortWithStatusJSON(http.StatusConflict, err.Error())
		case errors.As(err, &users.BadFormatError):
			c.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
		default:
			h.logger.Error().Stack().Err(err).Send()
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	c.Header("Authorization", token)
	c.JSON(http.StatusOK, token)
}

func (h *handler) LogoutFunc(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	var s *users.Session
	if err := h.ctrl.Logout(ctx, s); err != nil {
		h.logger.Error().Stack().Err(err).Send()
		// TO-DO
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	c.Redirect(http.StatusOK, "/")
}

func (h *handler) OrderFunc(c *gin.Context) {
	if ok := h.ValidateContentType(c, "text/plain"); !ok {
		return
	}

	s, ok := h.GetSessionHelper(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	if err := h.ctrl.Order(ctx, s.User(), c.Request.Body); err != nil {
		switch {
		case errors.As(err, &orders.InvalidRequestError):
			c.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
		case errors.As(err, &orders.InvalidNumberError):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, err.Error())
		case errors.As(err, &orders.AlreadyExistsError):
			c.AbortWithStatusJSON(http.StatusOK, err.Error())
		case errors.As(err, &orders.BelongsToAnotherError):
			c.AbortWithStatusJSON(http.StatusConflict, err.Error())
		default:
			h.logger.Error().Stack().Err(err).Send()
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	c.Status(http.StatusAccepted)

}

func (h *handler) OrdersFunc(c *gin.Context) {
	s, ok := h.GetSessionHelper(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()
	data, err := h.ctrl.Orders(ctx, s.User())
	if err != nil {
		switch {
		case errors.As(err, &orders.NoItemsError):
			c.AbortWithStatus(http.StatusNoContent)
		default:
			h.logger.Error().Stack().Err(err).Send()
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	ret, _ := json.Marshal(data)
	h.logger.Debug().Interface("data", string(ret)).Send()
	h.logger.Debug().Interface("data", string(ret)).Send()
	c.JSON(http.StatusOK, data)
}

func (h *handler) BalanceFunc(c *gin.Context) {
	s, ok := h.GetSessionHelper(c)
	if !ok {
		return
	}

	h.logger.Debug().Interface("balance", s.User().Balance).Send()
	c.JSON(http.StatusOK, s.User())
}

func (h *handler) WithdrawFunc(c *gin.Context) {
	if ok := h.ValidateContentType(c, "application/json"); !ok {
		return
	}

	s, ok := h.GetSessionHelper(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	if err := h.ctrl.Withdraw(ctx, s.User(), c.Request.Body); err != nil {
		switch {
		case errors.As(err, &withdrawals.InvalidOrderError):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, err.Error())
		case errors.As(err, &withdrawals.NotEnoughBalanceError):
			c.AbortWithStatusJSON(http.StatusPaymentRequired, err.Error())
		default:
			h.logger.Error().Stack().Err(err).Send()
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	c.Status(http.StatusOK)
}

func (h *handler) WithdrawalsFunc(c *gin.Context) {
	s, ok := h.GetSessionHelper(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()
	data, err := h.ctrl.Withdrawals(ctx, s.User())
	if err != nil {
		switch {
		case errors.As(err, &withdrawals.NoRecordsError):
			c.AbortWithStatus(http.StatusNoContent)
		default:
			h.logger.Error().Stack().Err(err).Send()
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, data)
}

func (h *handler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, "pong")
}

func (h *handler) GetSessionHelper(c *gin.Context) (*users.Session, bool) {
	data, ok := c.Get("session")
	if data == nil || !ok {
		h.logger.Error().Msg("middleware passed but session not found")
		c.AbortWithStatus(http.StatusInternalServerError)
		return nil, false
	}

	return data.(*users.Session), true
}

func (h *handler) ValidateContentType(c *gin.Context, t string) bool {
	ct := c.GetHeader("Content-type")
	if ct != t {
		c.AbortWithStatus(http.StatusBadRequest)
		return false
	}

	return true
}
