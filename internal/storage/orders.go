package storage

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/jackc/pgx/v4"
)

type Order struct {
	Number     string    `json:"number"`            // номер заказа
	Login      string    `json:"login"`             // логин пользователя, оформившего заказ
	Status     string    `json:"status"`            // статус обработки расчётов
	Accrual    int       `json:"accrual,omitempty"` // количество начисленных за заказ баллов
	UploadedAt time.Time `json:"uploaded_at"`       // время загрузки
}

// Добавляем новый заказ в базу
func (storage *Database) AddOrder(orderNumber, login, status string) error {
	_, err := storage.dbpool.Exec(context.Background(),
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

// извлекает заказ из базы
func (storage *Database) GetOrder(orderNumber string) (*Order, error) {
	row := storage.dbpool.QueryRow(context.Background(),
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
	rows, err := storage.dbpool.Query(context.Background(),
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