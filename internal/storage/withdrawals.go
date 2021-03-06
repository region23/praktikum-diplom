package storage

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	my_errors "github.com/region23/praktikum-diplom/internal/errors"
	"github.com/rs/zerolog/log"
)

type Withdraw struct {
	Order       string    `json:"order"`        // номер заказа
	Sum         float64   `json:"sum"`          // сумма списания в счет заказа 1 бал = 1 рубль (в копейках)
	ProcessedAt time.Time `json:"processed_at"` // время списания
}

type Balance struct {
	Current   float64 `json:"current"`   // текущая сумма балов лояльности
	Withdrawn float64 `json:"withdrawn"` // сумма использованных за весь период регистрации баллов
}

func (storage *Database) currentBalance(tx pgx.Tx, login string) (*Balance, error) {
	// получить общее количество баллов лояльности, накопленных за весь период
	row := tx.QueryRow(storage.Ctx,
		`SELECT COALESCE(SUM(accrual), 0) as sum FROM orders WHERE login = $1 AND status = $2`,
		login, StatusProcessed)

	var totalaccruals float64

	err := row.Scan(&totalaccruals)
	if err != nil {
		return nil, err
	}

	// сумма использованных за весь период регистрации баллов
	row2 := tx.QueryRow(storage.Ctx,
		`SELECT COALESCE(SUM(sum), 0) as sum FROM withdrawals WHERE login = $1`,
		login)

	var withdrawn float64

	err = row2.Scan(&withdrawn)
	if err != nil {
		return nil, err
	}

	current := totalaccruals - withdrawn

	balance := Balance{Current: current, Withdrawn: withdrawn}

	return &balance, nil
}

// Получение текущего баланса пользователя
func (storage *Database) CurrentBalance(login string) (*Balance, error) {
	tx, err := storage.dbpool.Begin(storage.Ctx)
	if err != nil {
		return nil, err
	}
	//defer tx.Rollback(storage.Ctx)
	balance, err := storage.currentBalance(tx, login)
	if err != nil {
		wrapped := fmt.Errorf("[functionName] error when getting current balance: %w", err)
		errRlbck := tx.Rollback(storage.Ctx)
		if errRlbck != nil {
			log.Error().Err(errRlbck).Msg("[functionName] error when rollback transaction in current balance")
		}

		return nil, wrapped
	}

	return balance, tx.Commit(storage.Ctx)
}

// Добавляем новое списание баллов
// sum - сумма списания в рублях
func (storage *Database) AddWithdraw(orderNumber string, login string, sum float64) error {
	// начать транзакцию
	tx, err := storage.dbpool.Begin(storage.Ctx)
	if err != nil {
		return err
	}
	//defer tx.Rollback(storage.Ctx)
	// считать текущий баланс пользователя
	// не понимаю как этот метод обернуть в транзакцию - он используется в нескольких местах
	balance, err := storage.currentBalance(tx, login)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get current balance from DB")
		return err
	}

	if sum >= balance.Current {
		return my_errors.ErrInsufficientBalance
	}

	// если баланса хватает для текущего списания - делаем списание
	_, err = tx.Exec(storage.Ctx,
		`INSERT INTO withdrawals (order_number, login, sum) VALUES ($1, $2, $3);`,
		orderNumber,
		login,
		sum)

	if err != nil {
		log.Error().Err(err).Msg("Unable to INSERT withdraw to DB")
		wrapped := fmt.Errorf("[functionName] error when getting current balance: %w", err)
		errRlbck := tx.Rollback(storage.Ctx)
		if errRlbck != nil {
			log.Error().Err(errRlbck).Msg("[functionName] error when rollback transaction in current balance")
		}
		return wrapped
	}

	return tx.Commit(storage.Ctx)
}

func (storage *Database) GetWithdrawals(login string) (*[]Withdraw, error) {
	rows, err := storage.dbpool.Query(storage.Ctx,
		`SELECT order_number, sum, processed_at FROM withdrawals WHERE login = $1 ORDER BY processed_at ASC`,
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
