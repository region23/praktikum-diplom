package storage

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

type Database struct {
	dbpool *pgxpool.Pool
}

func NewDatabase(dbpool *pgxpool.Pool) *Database {
	return &Database{
		dbpool: dbpool,
	}
}

// проверяем есть ли соединение с базой данных
func Ping(dbpool *pgxpool.Pool) error {
	if dbpool == nil {
		return errors.New("connection is nil")
	}

	err := dbpool.Ping(context.Background())
	if err != nil {
		return err
	}

	return nil
}

// При инициализации базы данных проверить, есть ли таблица metrics.
// Если её нет, то создать.
func InitDB(dbpool *pgxpool.Pool) error {
	query := `CREATE TABLE IF NOT EXISTS users (
		ID SERIAL PRIMARY KEY NOT NULL,
		login VARCHAR(100) UNIQUE NOT NULL,
		password VARCHAR(64) NOT NULL,
	  );`

	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()

	_, err := dbpool.Exec(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Error when creating product table")
		return err
	}

	//rows := res.RowsAffected()
	//log.Printf("Rows affected when creating table: %d", rows)
	return nil
}