package main

import (
	"fmt"
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/gophermart"
	"net/http"
)

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Println("hello")
}

func main() {
	cfg := config.NewServerConfig().Env().Flags()
	logger := cfg.GetLogger()

	logger.Debug().Interface("Config", cfg).Send()
	logger.Info().Msg("Starting server")
	defer logger.Info().Msg("Server closed")
	gophermart.Run(cfg, logger)
}
