package main

import (
	"flag"
	"net/http"

	"github.com/caarlos0/env/v6"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/region23/praktikum-diplom/internal/server"
	"github.com/region23/praktikum-diplom/internal/storage"
	"github.com/rs/zerolog/log"
)

var dbpool *pgxpool.Pool

type Config struct {
	RunAddress           string `env:"RUN_ADDRESS"`
	DatabaseURI          string `env:"DATABASE_URI"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

var cfg Config = Config{}

func init() {
	flag.StringVar(&cfg.RunAddress, "a", "127.0.0.1:8080", "server address")
	flag.StringVar(&cfg.DatabaseURI, "d", "", "database connection string")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", "", "адрес системы расчёта начислений")
}

func main() {
	flag.Parse()

	if err := env.Parse(&cfg); err != nil {
		log.Error().Err(err).Msgf("%+v\n", err)
	}

	var repository storage.Repository

	log.Debug().Msg("Starting server...")

	srv := server.New(repository, dbpool)
	srv.MountHandlers()

	http.ListenAndServe(cfg.RunAddress, srv.Router)
}
