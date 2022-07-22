package externalapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	my_errors "github.com/region23/praktikum-diplom/internal/errors"
	"github.com/region23/praktikum-diplom/internal/storage"
)

type AccuralType struct {
	Order   string  `json:"order"`             // номер заказа
	Status  string  `json:"status"`            // статус расчёта начисления
	Accrual float64 `json:"accrual,omitempty"` // рассчитанные баллы к начислению, при отсутствии начисления — поле отсутствует в ответе
}

// получение информации о расчёте начислений баллов лояльности
func getOrderAccrual(ctx context.Context, httpClient *http.Client, accrualSystemAddress, number string) (accuralType *AccuralType, err error) {
	url := accrualSystemAddress + "/api/orders/" + number

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	request.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, err
	}

	// отправляем запрос
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	// успешная обработка запроса
	if response.StatusCode == http.StatusOK {
		var accuralType AccuralType
		err := json.NewDecoder(response.Body).Decode(&accuralType)
		if err != nil {
			return nil, err
		}

		return &accuralType, nil
	}

	// превышено количество запросов к сервису
	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := response.Header.Get("Retry-After")
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		if s, err := strconv.Atoi(retryAfter); err == nil {
			retryError := my_errors.RetryAfterError{RetryAfter: time.Duration(s), Err: errors.New(string(body))}
			return nil, &retryError
		}
	}

	// внутренняя ошибка сервера
	if response.StatusCode == http.StatusInternalServerError {
		return nil, my_errors.ErrInternalServerError
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("код ответа: %v. Чё вообще происходит: %v", response.StatusCode, body)

}

// Обновлений начислений и статусов начислений по заказам
func UpdateAccurals(ctx context.Context, httpClient *http.Client, storage *storage.Database, accrualSystemAddress string) error {
	// получаем список всех заказов со статусами NEW, REGISTERED, PROCESSING
	orders, err := storage.GetOrdersForUpdate()
	if err != nil {
		return err
	}

	sleep := 1 * time.Nanosecond
	// проходим в цикле по списку и получаем из удаленного сервиса обновления
	for _, order := range *orders {
		time.Sleep(sleep)

		accural, err := getOrderAccrual(ctx, httpClient, accrualSystemAddress, order.Number)
		if err != nil {
			retryAfter := new(my_errors.RetryAfterError)
			if errors.As(err, &retryAfter) {
				sleep = retryAfter.RetryAfter * time.Second
				continue
			}
		}

		if err != nil {
			return err
		}

		// обновляем данные по заказу в orders
		err = storage.UpdateOrder(accural.Order, accural.Status, accural.Accrual)
		if err != nil {
			return err
		}
		sleep = 1 * time.Nanosecond
	}

	return nil
}
