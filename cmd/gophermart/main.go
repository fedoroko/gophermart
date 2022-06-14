package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("SERVERING")
	hello := func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("hello")
	}
	http.ListenAndServe("localhost:8080", http.HandlerFunc(hello))
	//fmt.Println("server started !!!!!!")
	////time.Sleep(time.Second * 5)
	//fmt.Println("server started @@@@")
	//cfg := config.NewServerConfig().Env().Flags()
	//logger := cfg.GetLogger()
	//
	//logger.Debug().Interface("Config", cfg).Send()
	//logger.Info().Msg("Starting server")
	//defer logger.Info().Msg("Server closed")
	//gophermart.Run(cfg, logger)
	fmt.Println("server started !!!!!!")
}
