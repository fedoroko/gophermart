package gophermart

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fedoroko/gophermart/internal/accrual"
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/handlers"
	"github.com/fedoroko/gophermart/internal/middlewares"
	"github.com/fedoroko/gophermart/internal/storage"
)

func Run(cfg *config.ServerConfig, logger *config.Logger) {
	db, err := storage.Postgres(cfg, logger)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	q := accrual.NewQueue(cfg, db, logger)
	defer q.Close()
	go func() {
		if err = q.Listen(); err != nil {
			logger.Error().Caller().Stack().Err(err).Send()
		}
	}()

	r := router(db, q, logger)
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: r,
	}

	defer server.Close()
	go func() {
		logger.Info().Msg("Server started at " + cfg.Address)
		if err = server.ListenAndServe(); err != nil {
			logger.Error().Caller().Stack().Err(err).Msg("server error")
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("server shutdown err")
	}

	select {
	case <-ctx.Done():
		logger.Warn().Msg("server shutdown 5s timeout")
	}
}

func router(db storage.Repo, q accrual.Queue, logger *config.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middlewares.InstanceID(1))
	r.Use(middlewares.RateLimit())
	r.Use(middlewares.StatCollector())

	h := handlers.Handler(db, q, logger, time.Second*30)
	authBasic := middlewares.AuthBasic(db, logger)
	authWithBalance := middlewares.AuthWithBalance(db, logger)

	r.GET("/ping", h.Ping)
	api := r.Group("api/user")
	{
		api.POST("/login", h.LoginFunc)
		api.POST("/register", h.RegisterFunc)
		api.POST("/logout", authBasic, h.LogoutFunc)

		orders := api.Group("/orders")
		orders.Use(authBasic)
		{
			orders.POST("/", h.OrderFunc)
			orders.GET("/", h.OrdersFunc)
		}

		balance := api.Group("/balance")
		balance.Use(authWithBalance)
		{
			balance.GET("/", h.BalanceFunc)
			balance.POST("/withdraw", h.WithdrawFunc)
			balance.GET("/withdrawals", h.WithdrawalsFunc)
		}
	}

	return r
}
