package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/region23/praktikum-diplom/internal/storage"
)

type Server struct {
	storage storage.Repository
	Router  *chi.Mux
	DBPool  *pgxpool.Pool
}

func New(storage storage.Repository, dbpool *pgxpool.Pool) *Server {
	return &Server{
		storage: storage,
		Router:  chi.NewRouter(),
		DBPool:  dbpool,
	}
}

func (s *Server) MountHandlers() {
	// Mount all Middleware here
	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.StripSlashes)
	s.Router.Use(middleware.Compress(5))
	s.Router.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	s.Router.Use(middleware.Timeout(60 * time.Second))

	// Mount all handlers here
	s.Router.Post("/api/user/register", s.userRegister)
	s.Router.Post("/api/user/login", s.userLogin)
	s.Router.Post("/api/user/orders", s.postUserOrders)
	s.Router.Get("/api/user/orders", s.getUserOrders)
	s.Router.Get("/api/user/balance", s.getUserBalance)
	s.Router.Post("/api/user/balance/withdraw", s.userBalanceWithdraw)
	s.Router.Get("/api/user/balance/withdrawals", s.userBalanceWithdrawals)

}

// регистрация пользователя
func (s *Server) userRegister(w http.ResponseWriter, r *http.Request) {}

// аутентификация пользователя
func (s *Server) userLogin(w http.ResponseWriter, r *http.Request) {}

// загрузка пользователем номера заказа для расчёта
func (s *Server) postUserOrders(w http.ResponseWriter, r *http.Request) {}

// получение списка загруженных пользователем номеров заказов, статусов их обработки и информации о начислениях
func (s *Server) getUserOrders(w http.ResponseWriter, r *http.Request) {}

// получение текущего баланса счёта баллов лояльности пользователя
func (s *Server) getUserBalance(w http.ResponseWriter, r *http.Request) {}

// запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа
func (s *Server) userBalanceWithdraw(w http.ResponseWriter, r *http.Request) {}

// получение информации о выводе средств с накопительного счёта пользователем
func (s *Server) userBalanceWithdrawals(w http.ResponseWriter, r *http.Request) {}
