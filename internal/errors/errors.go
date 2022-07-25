package errors

import (
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrAlreadyExists       = errors.New("already exists")
	ErrInsufficientBalance = errors.New("сумма списания больше текущей суммы")
	ErrInternalServerError = errors.New("InternalServerError")
)

type RetryAfterError struct {
	RetryAfter time.Duration
	Err        error
}

func (e *RetryAfterError) Error() string {
	return e.RetryAfter.String()
}

func (e *RetryAfterError) Unwrap() error {
	return e.Err
}
