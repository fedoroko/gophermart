package config

import (
	"flag"
	"os"

	"github.com/caarlos0/env/v6"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

type Config interface {
	Flags() *Config
	Env() *Config
}

type ServerConfig struct {
	Address      string `env:"RUN_ADDRESS"`
	Accrual      string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	Database     string `env:"DATABASE_URI"`
	DBRefresh    bool   `env:"DB_REFRESH"`
	Debug        bool
	InstanceID   int    `env:"INSTANCE_ID"`
	RabbitMQ     string `env:"RABBITMQ_URI"`
	SetupAccrual bool
	WorkersCount int `env:"WORKERS_COUNT"`
}

func (s *ServerConfig) Flags() *ServerConfig {
	flag.StringVar(&s.Address, "a", s.Address, "Host address")
	flag.StringVar(&s.Accrual, "r", s.Accrual, "Accrual address")
	flag.StringVar(&s.Database, "d", s.Database, "Database DSN")
	flag.BoolVar(&s.DBRefresh, "dbr", s.DBRefresh, "Refresh database")
	flag.BoolVar(&s.Debug, "debug", s.Debug, "Debug mode")
	flag.StringVar(&s.RabbitMQ, "mq", s.RabbitMQ, "Rabbitmq DSN")
	flag.BoolVar(&s.SetupAccrual, "sa", s.SetupAccrual, "Set up accrual")
	flag.IntVar(&s.WorkersCount, "w", s.WorkersCount, "Num of workers")
	flag.Parse()

	return s
}

func (s *ServerConfig) Env() *ServerConfig {
	err := env.Parse(s)
	if err != nil {
		log.Err(err).Send()
	}

	return s
}

func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		Address:      "127.0.0.1:8000",
		Accrual:      "127.0.0.1:8080",
		Database:     "postgresql://fedoroko@localhost/gophermart",
		DBRefresh:    false,
		Debug:        false,
		InstanceID:   1,
		SetupAccrual: false,
		WorkersCount: 2,
	}
}

type Logger struct {
	*zerolog.Logger
}

func TestLogger() *Logger {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: zerolog.TimeFormatUnix}
	logger := zerolog.New(output).With().Timestamp().Logger()

	return &Logger{
		Logger: &logger,
	}
}

func NewLogger(logger *zerolog.Logger) *Logger {
	return &Logger{
		Logger: logger,
	}
}

func (s *ServerConfig) GetLogger() *Logger {
	logLevel := zerolog.InfoLevel
	gin.SetMode(gin.ReleaseMode)
	if s.Debug {
		logLevel = zerolog.DebugLevel
		gin.SetMode(gin.DebugMode)
	}

	zerolog.SetGlobalLevel(logLevel)
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: zerolog.TimeFormatUnix}
	logger := zerolog.New(output).With().Timestamp().Logger()

	return NewLogger(&logger)
}
