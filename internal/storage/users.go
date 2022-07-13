package storage

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/jackc/pgx/v4"
)

type User struct {
	ID       string `json:"id,omitempty"` // ID пользователя
	Login    string `json:"login"`        // логин пользователя
	Password string `json:"password"`     // хэш пароля SHA-256
}

// проверяем, есть ли пользователь с таким логином в базе
func (storage *Database) UserExist(login, hashedPassword string) (bool, error) {
	var row pgx.Row
	if hashedPassword != "" {
		row = storage.dbpool.QueryRow(context.Background(),
			`SELECT count(*) FROM users WHERE login = $1 AND password = $2`,
			login, hashedPassword)
	} else {
		row = storage.dbpool.QueryRow(context.Background(),
			`SELECT count(*) FROM users WHERE login = $1`,
			login)
	}

	var userCount int

	err := row.Scan(&userCount)

	switch err {
	case nil:
		if userCount > 0 {
			return true, nil
		} else {
			return false, nil
		}

	case pgx.ErrNoRows:
		return false, pgx.ErrNoRows
	default:
		return false, err
	}

}

// извлекает пользователя из базы
func (storage *Database) GetUser(login string) (*User, error) {
	row := storage.dbpool.QueryRow(context.Background(),
		`SELECT id, login, password FROM users WHERE login = $1`,
		login)

	var user User

	err := row.Scan(&user.ID, &user.Login, &user.Password)

	switch err {
	case nil:
		return &user, nil
	case pgx.ErrNoRows:
		return nil, pgx.ErrNoRows
	default:
		return nil, err
	}

}

// добавляет пользователя в базу
func (storage *Database) AddUser(user *User) error {
	_, err := storage.dbpool.Exec(context.Background(),
		`INSERT INTO users (login, password) VALUES ($1, $2);`,
		user.Login,
		user.Password)

	if err != nil {
		log.Error().Err(err).Msg("Unable to INSERT user to DB")
		return err
	}

	return nil
}
