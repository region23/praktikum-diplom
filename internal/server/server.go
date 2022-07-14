package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"crypto/sha256"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joeljunstrom/go-luhn"
	"github.com/region23/praktikum-diplom/internal/storage"
	"github.com/rs/zerolog/log"
)

type Server struct {
	storage   storage.Database
	Router    *chi.Mux
	DBPool    *pgxpool.Pool
	TokenAuth *jwtauth.JWTAuth
}

func New(storage storage.Database, dbpool *pgxpool.Pool, tokenAuth *jwtauth.JWTAuth) *Server {
	return &Server{
		storage:   storage,
		Router:    chi.NewRouter(),
		DBPool:    dbpool,
		TokenAuth: tokenAuth,
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

	// Public routes
	s.Router.Group(func(r chi.Router) {
		r.Post("/api/user/register", s.userRegister)
		r.Post("/api/user/login", s.userLogin)
	})

	s.Router.Group(func(r chi.Router) {
		// Seek, verify and validate JWT tokens
		r.Use(jwtauth.Verifier(s.TokenAuth))

		// Handle valid / invalid tokens. In this example, we use
		// the provided authenticator middleware, but you can write your
		// own very easily, look at the Authenticator method in jwtauth.go
		// and tweak it, its not scary.
		r.Use(jwtauth.Authenticator)

		r.Post("/api/user/orders", s.postUserOrders)
		r.Get("/api/user/orders", s.getUserOrders)
		r.Get("/api/user/balance", s.getUserBalance)
		r.Post("/api/user/balance/withdraw", s.userBalanceWithdraw)
		r.Get("/api/user/balance/withdrawals", s.userBalanceWithdrawals)
	})
}

// регистрация пользователя
func (s *Server) userRegister(w http.ResponseWriter, r *http.Request) {
	// Возможные коды ответа:
	// 200 — пользователь успешно зарегистрирован и аутентифицирован;
	// 400 — неверный формат запроса;
	// 409 — логин уже занят;
	// 500 — внутренняя ошибка сервера.

	// декодировать логин и пароль, переданные в json
	var user storage.User

	// decode input or return error
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		respBody := ResponseBody{Error: fmt.Sprint("Decode error! please check your JSON formating.", err.Error())}
		JSONResponse(w, respBody, http.StatusBadRequest)
		return
	}

	// проверить, есть ли такой логин в базе. Если есть возвращаем 409
	userExist, err := s.storage.UserExist(user.Login, "")

	if err != nil {
		respBody := ResponseBody{Error: fmt.Sprintf("Ошибка при получении пользователя: %v", err.Error())}
		JSONResponse(w, respBody, http.StatusInternalServerError)
		return
	}

	if userExist {
		respBody := ResponseBody{Error: "логин уже занят"}
		JSONResponse(w, respBody, http.StatusConflict)
		return
	}

	// хэшируем пароль и регистрируем пользователя
	hashedPassword := fmt.Sprintf("%x", sha256.Sum256([]byte(user.Password)))
	user.Password = hashedPassword
	// если нет, добавляем в базу и возвращаем 200 и jwt-token
	err = s.storage.AddUser(&user)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка при получении пользователя: %v", err.Error()), http.StatusInternalServerError)
		return
	}
	_, tokenString, _ := s.TokenAuth.Encode(map[string]interface{}{"user_id": user.Login})
	log.Debug().Msg(tokenString)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Authorization", fmt.Sprintf("BEARER %v", tokenString))

	cookie := &http.Cookie{
		Name:   "jwt",
		Value:  tokenString,
		MaxAge: 3600,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(nil)

}

// аутентификация пользователя
func (s *Server) userLogin(w http.ResponseWriter, r *http.Request) {
	// Возможные коды ответа:
	// 200 — пользователь успешно аутентифицирован;
	// 400 — неверный формат запроса;
	// 401 — неверная пара логин/пароль;
	// 500 — внутренняя ошибка сервера.

	// декодировать логин и пароль, переданные в json
	var user storage.User

	// decode input or return error
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		respBody := ResponseBody{Error: fmt.Sprint("Decode error! please check your JSON formating.", err.Error())}
		JSONResponse(w, respBody, http.StatusBadRequest)
		return
	}

	// Хэшируем пароль и проверяем в базе пару ключ/хэш пароля. Если ок 200 и jwt-токен
	hashedPassword := fmt.Sprintf("%x", sha256.Sum256([]byte(user.Password)))
	user.Password = hashedPassword
	userExist, err := s.storage.UserExist(user.Login, user.Password)

	if err != nil {
		respBody := ResponseBody{Error: fmt.Sprintf("Ошибка при получении пользователя: %v", err.Error())}
		JSONResponse(w, respBody, http.StatusInternalServerError)
		return
	}

	if !userExist {
		respBody := ResponseBody{Error: "неверная пара логин/пароль"}
		JSONResponse(w, respBody, http.StatusUnauthorized)
		return
	}

	_, tokenString, _ := s.TokenAuth.Encode(map[string]interface{}{"user_id": user.Login})
	log.Debug().Msg(tokenString)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Authorization", fmt.Sprintf("BEARER %v", tokenString))

	cookie := &http.Cookie{
		Name:   "jwt",
		Value:  tokenString,
		MaxAge: 3600,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(nil)
}

// загрузка пользователем номера заказа для расчёта
func (s *Server) postUserOrders(w http.ResponseWriter, r *http.Request) {
	// Возможные коды ответа:
	// 200 — номер заказа уже был загружен этим пользователем;
	// 202 — новый номер заказа принят в обработку;
	// 400 — неверный формат запроса;
	// 401 — пользователь не аутентифицирован;
	// 409 — номер заказа уже был загружен другим пользователем;
	// 422 — неверный формат номера заказа;
	// 500 — внутренняя ошибка сервера.

	_, claims, err := jwtauth.FromContext(r.Context())
	if err != nil {
		respBody := ResponseBody{Error: fmt.Sprintf("внутренняя ошибка сервера: %v", err.Error())}
		JSONResponse(w, respBody, http.StatusInternalServerError)
		return
	}

	currentLogin, _ := claims["user_id"].(string)

	// проверить номер заказ алгоритмом Луна
	orderNumber, err := io.ReadAll(r.Body)
	if err != nil {
		respBody := ResponseBody{Error: fmt.Sprintf("неверный формат запроса: %v", err.Error())}
		JSONResponse(w, respBody, http.StatusBadRequest)
		return
	}

	valid := luhn.Valid(string(orderNumber))

	if !valid {
		respBody := ResponseBody{Error: "неверный формат номера заказа"}
		JSONResponse(w, respBody, http.StatusUnprocessableEntity)
		return
	}

	// смотрим, есть ли такой номер заказа уже в базе и смотрим добавлен он текущим пользователем или другим
	// и возвращаем соответствующую ошибку
	order, err := s.storage.GetOrder(string(orderNumber))

	if err != nil && err != pgx.ErrNoRows {
		respBody := ResponseBody{Error: fmt.Sprintf("внутренняя ошибка сервера: %v", err.Error())}
		JSONResponse(w, respBody, http.StatusInternalServerError)
		return
	}

	// Такой номер заказа не найден - можно добавить новый
	if err == pgx.ErrNoRows {
		err := s.storage.AddOrder(string(orderNumber), currentLogin, "NEW")
		if err != nil {
			respBody := ResponseBody{Error: fmt.Sprintf("при загрузке заказа произошла ошибка: %v", err.Error())}
			JSONResponse(w, respBody, http.StatusInternalServerError)
			return
		}

		respBody := ResponseBody{Success: "новый номер заказа принят в обработку"}
		JSONResponse(w, respBody, http.StatusAccepted)
		return
	}

	if order.Login == currentLogin {
		respBody := ResponseBody{Success: "номер заказа уже был загружен этим пользователем"}
		JSONResponse(w, respBody, http.StatusOK)
		return
	}

	if order.Login != currentLogin {
		respBody := ResponseBody{Error: "номер заказа уже был загружен другим пользователем"}
		JSONResponse(w, respBody, http.StatusConflict)
		return
	}
}

// получение списка загруженных пользователем номеров заказов, статусов их обработки и информации о начислениях
func (s *Server) getUserOrders(w http.ResponseWriter, r *http.Request) {
	_, claims, err := jwtauth.FromContext(r.Context())
	if err != nil {
		respBody := ResponseBody{Error: fmt.Sprintf("внутренняя ошибка сервера: %v", err.Error())}
		JSONResponse(w, respBody, http.StatusInternalServerError)
		return
	}

	currentLogin, _ := claims["user_id"].(string)

	orders, err := s.storage.GetOrders(currentLogin)

	if err != nil && err != pgx.ErrNoRows {
		respBody := ResponseBody{Error: fmt.Sprintf("внутренняя ошибка сервера: %v", err.Error())}
		JSONResponse(w, respBody, http.StatusInternalServerError)
		return
	}

	// У пользователя нет заказов
	if err == pgx.ErrNoRows {
		respBody := ResponseBody{Success: "нет данных для ответа"}
		JSONResponse(w, respBody, http.StatusNoContent)
		return
	}

	JSONResponse(w, orders, http.StatusOK)
}

// получение текущего баланса счёта баллов лояльности пользователя
func (s *Server) getUserBalance(w http.ResponseWriter, r *http.Request) {}

// запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа
func (s *Server) userBalanceWithdraw(w http.ResponseWriter, r *http.Request) {}

// получение информации о выводе средств с накопительного счёта пользователем
func (s *Server) userBalanceWithdrawals(w http.ResponseWriter, r *http.Request) {}

type ResponseBody struct {
	Success string `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
}

func JSONResponse(w http.ResponseWriter, responseStruct interface{}, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(responseStruct)
}
