package storage

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/jackc/pgx/v4"
)

type Order struct {
	Number     string    `json:"number"`            // номер заказа
	Login      string    `json:"login"`             // логин пользователя, оформившего заказ
	Status     string    `json:"status"`            // статус обработки расчётов
	Accrual    float64   `json:"accrual,omitempty"` // количество начисленных за заказ баллов
	UploadedAt time.Time `json:"uploaded_at"`       // время загрузки
}

// Добавляем новый заказ в базу
func (storage *Database) AddOrder(orderNumber, login, status string) error {
	_, err := storage.dbpool.Exec(storage.Ctx,
		`INSERT INTO orders (number, login, status) VALUES ($1, $2, $3);`,
		orderNumber,
		login,
		status)

	if err != nil {
		log.Error().Err(err).Msg("Unable to INSERT order to DB")
		return err
	}

	return nil
}

func (storage *Database) UpdateOrder(orderNumber string, status string, accrual float64) error {
	_, err := storage.dbpool.Exec(storage.Ctx,
		`UPDATE orders SET status = $1, accrual = $2 WHERE number = $3;`,
		status,
		accrual,
		orderNumber)

	if err != nil {
		log.Error().Err(err).Msg("Unable to UPDATE order in DB")
		return err
	}

	return nil
}

// извлекает заказ из базы
func (storage *Database) GetOrder(orderNumber string) (*Order, error) {
	row := storage.dbpool.QueryRow(storage.Ctx,
		`SELECT number, login, status, accrual, uploaded_at FROM orders WHERE number = $1`,
		orderNumber)

	var order Order

	err := row.Scan(&order.Number, &order.Login, &order.Status, &order.Accrual, &order.UploadedAt)

	switch err {
	case nil:
		return &order, nil
	case pgx.ErrNoRows:
		return nil, pgx.ErrNoRows
	default:
		return nil, err
	}
}

// извлекает все заказы пользователя из базы
func (storage *Database) GetOrders(login string) (*[]Order, error) {
	rows, err := storage.dbpool.Query(storage.Ctx,
		`SELECT number, login, status, accrual, uploaded_at FROM orders WHERE login = $1 ORDER BY uploaded_at ASC`,
		login)

	if err != nil {
		return nil, err
	}

	var orders []Order

	for rows.Next() {
		var order Order
		err := rows.Scan(&order.Number, &order.Login, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return &orders, rows.Err()
}

// извлекает все заказы всех пользователей из базы, требующие обновление статуса и начислений
func (storage *Database) GetOrdersForUpdate() (*[]Order, error) {
	rows, err := storage.dbpool.Query(storage.Ctx,
		`SELECT number, login, status, accrual, uploaded_at FROM orders WHERE status IN ('NEW', 'REGISTERED', 'PROCESSING') ORDER BY uploaded_at ASC`)

	if err != nil {
		return nil, err
	}

	var orders []Order

	for rows.Next() {
		var order Order
		err := rows.Scan(&order.Number, &order.Login, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return &orders, rows.Err()
}
