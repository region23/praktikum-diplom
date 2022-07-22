package main

import (
	"context"
	"errors"
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

func createChannel() (chan os.Signal, func()) {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	return stopCh, func() {
		close(stopCh)
	}
}

func start(server *http.Server) {
	log.Info().Msg("application started")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	} else {
		log.Info().Msg("application stopped gracefully")
	}
}

func shutdown(ctx context.Context, cancel context.CancelFunc, server *http.Server) {
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		panic(err)
	} else {
		log.Info().Msg("application shutdowned")
	}
}

func main() {
	var dbpool *pgxpool.Pool

	flag.Parse()

	if err := env.Parse(&cfg); err != nil {
		log.Error().Err(err).Msgf("%+v\n", err)
	}

	var repository *storage.Database

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Инициализируем подключение к базе данных
	var err error
	dbpool, err = pgxpool.Connect(ctx, cfg.DatabaseURI)
	if err != nil {
		log.Fatal().Err(err).Msg("Не смогли подключиться к базе данных")
	}

	err = storage.InitDB(ctx, dbpool)
	if err != nil {
		log.Fatal().Err(err).Msg("Не смогли подключиться к базе данных")
	}

	defer dbpool.Close()

	repository = storage.NewDatabase(ctx, dbpool)

	srv := server.New(*repository, tokenAuth)
	srv.MountHandlers()

	httpServer := &http.Server{Addr: cfg.RunAddress, Handler: srv.Router}
	go start(httpServer)

	stopCh, closeCh := createChannel()
	defer closeCh()

	httpClient := http.Client{Timeout: 5 * time.Second}

	go func(ctx context.Context, httpClient *http.Client) {
	updateAccuralsBlock:
		for {
			select {
			case <-stopCh:
				log.Info().Msg("завершили UpdateAccurals")
				break updateAccuralsBlock
			default:
				err = externalapi.UpdateAccurals(ctx, httpClient, repository, cfg.AccrualSystemAddress)
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						log.Debug().Err(err).Msg("Внешний сервис не доступен")
						time.Sleep(10 * time.Second)
					} else {
						log.Debug().Err(err).Msg("При доступе к внешнему сервису произошла ошибка")
					}
				}
			}
		}
	}(ctx, &httpClient)

	log.Info().Msgf("notified: %v", <-stopCh)

	shutdown(ctx, cancel, httpServer)
}
