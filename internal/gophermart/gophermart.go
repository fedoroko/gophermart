package gophermart

import (
	"github.com/fedoroko/gophermart/internal/accrual"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/handlers"
	"github.com/fedoroko/gophermart/internal/storage"
)

func Run(cfg *config.ServerConfig, logger *config.Logger) {
	db, err := storage.Postgres(cfg, logger)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	//q := accrual.NewQueue(cfg, db, logger)
	//defer q.Close()
	//go func() {
	//	if err = q.Listen(); err != nil {
	//		logger.Error().Stack().Err(err).Send()
	//	}
	//}()

	r := router(db, nil, logger)
	server := &http.Server{
		Addr:    "localhost:8080",
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

func router(db storage.Repo, q accrual.Queue, logger *config.Logger) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Compress(5))

	h := handlers.Handler(db, q, logger, time.Second*30)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/login", h.LoginFunc)
		r.Post("/register", h.RegisterFunc)
		r.Post("/logout", h.LoginFunc)
		//r.Route("orders/", func(r chi.Router) {
		//	r.Post("/", h.OrderFunc)
		//	r.Get("/", h.OrdersFunc)
		//})
		//r.Route("/balance", func(r chi.Router) {
		//	r.Get("/", h.BalanceFunc)
		//	r.Post("/withdrawal", h.WithdrawFunc)
		//	r.Get("/withdrawals", h.WithdrawalsFunc)
		//})
	})

	return r
}
