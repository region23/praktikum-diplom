package storage

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type Withdraw struct {
	Order       string    `json:"order"`        // номер заказа
	Sum         int       `json:"sum"`          // сумма списания в счет заказа 1 бал = 1 рубль (в копейках)
	ProcessedAt time.Time `json:"processed_at"` // время списания
}

type Balance struct {
	Current   float64 `json:"current"`   // текущая сумма балов лояльности
	Withdrawn float64 `json:"withdrawn"` // сумма использованных за весь период регистрации баллов
}

// Получение текущего баланса пользователя
func (storage *Database) CurrentBalance(login string) (*Balance, error) {
	// получить общее количество баллов лояльности, накопленных за весь период
	row := storage.dbpool.QueryRow(context.Background(),
		`SELECT SUM(accrual) FROM orders WHERE login = $1 AND status = "PROCESSED"`,
		login)

	var totalaccruals int

	err := row.Scan(&totalaccruals)
	if err != nil {
		return nil, err
	}

	// сумма использованных за весь период регистрации баллов
	row2 := storage.dbpool.QueryRow(context.Background(),
		`SELECT SUM(sum) FROM withdrawals WHERE login = $1`,
		login)

	var withdrawn int

	err = row2.Scan(&withdrawn)
	if err != nil {
		return nil, err
	}

	current := float64(totalaccruals*100-withdrawn) / 100

	balance := Balance{Current: current, Withdrawn: float64(withdrawn)}

	return &balance, nil
}

// Добавляем новое списание баллов
// sum - сумма списания в рублях
func (storage *Database) AddWithdraw(orderNumber string, login string, sum float64) error {
	// начать транзакцию
	// считать текущий баланс пользователя
	balance, err := storage.CurrentBalance(login)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get current balance from DB")
		return err
	}

	if sum >= balance.Current {
		return ErrInsufficientBalance
	}

	// если баланса хватает для текущего списания - делаем списание
	_, err = storage.dbpool.Exec(context.Background(),
		`INSERT INTO withdrawals (order, login, sum) VALUES ($1, $2, $3);`,
		orderNumber,
		login,
		sum*100)

	if err != nil {
		log.Error().Err(err).Msg("Unable to INSERT withdraw to DB")
		return err
	}

	return nil
}

func (storage *Database) GetWithdrawals(login string) (*[]Withdraw, error) {
	rows, err := storage.dbpool.Query(context.Background(),
		`SELECT order, sum, processed_at FROM withdrawals WHERE login = $1 ORDER BY processed_at ASC`,
		login)

	if err != nil {
		return nil, err
	}

	var withdrawals []Withdraw

	for rows.Next() {
		var withdraw Withdraw
		err := rows.Scan(&withdraw.Order, &withdraw.Sum, &withdraw.ProcessedAt)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, withdraw)
	}

	return &withdrawals, rows.Err()
}
