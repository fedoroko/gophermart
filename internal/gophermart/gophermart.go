package gophermart

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

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

	r := router(db, logger)
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: r,
	}

	defer server.Close()
	go func() {
		logger.Info().Msg("Server started at " + cfg.Address)
		if err = server.ListenAndServe(); err != nil {
			logger.Error().Stack().Err(err).Send()
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	<-sig
}

func router(db storage.Repo, logger *config.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middlewares.InstanceID(1))
	r.Use(middlewares.RateLimit())

	h := handlers.Handler(db, logger, time.Second*30)
	r.GET("/ping", h.Ping)
	auth := middlewares.AuthRequired(db, logger)

	api := r.Group("api/user")
	{
		api.POST("/login", h.LoginFunc)
		api.POST("/register", h.RegisterFunc)
		api.POST("/logout", auth, h.LogoutFunc)

		orders := api.Group("/orders")
		orders.Use(auth)
		{
			orders.POST("/", h.OrderFunc)
			orders.GET("/", h.OrdersFunc)
		}

		balance := api.Group("/balance")
		balance.Use(auth)
		{
			balance.GET("/", h.BalanceFunc)
			balance.POST("/withdraw", h.WithdrawFunc)
			balance.GET("/withdrawals", h.WithdrawalsFunc)
		}
	}

	return r
}
