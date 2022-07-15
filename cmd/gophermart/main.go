package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v4/pgxpool"
	externalapi "github.com/region23/praktikum-diplom/internal/external_api"
	"github.com/region23/praktikum-diplom/internal/server"
	"github.com/region23/praktikum-diplom/internal/storage"
	"github.com/rs/zerolog/log"
)

var dbpool *pgxpool.Pool
var tokenAuth *jwtauth.JWTAuth

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

	tokenAuth = jwtauth.New("HS256", []byte("secret"), nil)
}

func main() {
	flag.Parse()

	if err := env.Parse(&cfg); err != nil {
		log.Error().Err(err).Msgf("%+v\n", err)
	}

	osSigChan := make(chan os.Signal, 1)
	signal.Notify(osSigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var repository *storage.Database

	// Инициализируем подключение к базе данных
	var err error
	dbpool, err = pgxpool.Connect(context.Background(), cfg.DatabaseURI)
	if err != nil {
		log.Fatal().Err(err).Msg("Не смогли подключиться к базе данных")
	}

	err = storage.InitDB(dbpool)
	if err != nil {
		log.Fatal().Err(err).Msg("Не смогли подключиться к базе данных")
	}

	defer dbpool.Close()

	go func() {
		for {
			select {
			case <-osSigChan:
				os.Exit(0)
			default:
				err = externalapi.UpdateAccurals(repository, cfg.AccrualSystemAddress)
				if err != nil {
					log.Debug().Err(err).Msg("При доступе к внешнему сервису произошла ошибка")
					time.Sleep(1 * time.Second)
				}
			}
		}
	}()

	repository = storage.NewDatabase(dbpool)

	log.Debug().Msg("Starting server...")

	srv := server.New(*repository, dbpool, tokenAuth)
	srv.MountHandlers()

	http.ListenAndServe(cfg.RunAddress, srv.Router)
}
