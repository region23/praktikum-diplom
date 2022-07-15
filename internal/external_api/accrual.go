package externalapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/region23/praktikum-diplom/internal/storage"
)

type AccuralType struct {
	Order   string `json:"order"`             // номер заказа
	Status  string `json:"status"`            // статус расчёта начисления
	Accrual int    `json:"accrual,omitempty"` // рассчитанные баллы к начислению, при отсутствии начисления — поле отсутствует в ответе
}

// получение информации о расчёте начислений баллов лояльности
func getOrderAccrual(accrualSystemAddress, number string) (accuralType *AccuralType, retryAfter int, err error) {
	url := accrualSystemAddress + "/api/orders/" + number

	request, err := http.NewRequest(http.MethodGet, url, nil)
	request.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, 0, err
	}

	client := &http.Client{}
	// отправляем запрос
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}

	defer response.Body.Close()

	// успешная обработка запроса
	if response.StatusCode == http.StatusOK {
		var accuralType AccuralType
		err := json.NewDecoder(response.Body).Decode(&accuralType)
		if err != nil {
			return nil, 0, err
		}

		return &accuralType, 0, nil
	}

	// превышено количество запросов к сервису
	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := response.Header.Get("Retry-After")
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, 0, err
		}

		if s, err := strconv.Atoi(retryAfter); err == nil {
			return nil, s, errors.New(string(body))
		}
	}

	// внутренняя ошибка сервера
	if response.StatusCode == http.StatusInternalServerError {
		return nil, 0, storage.ErrInternalServerError
	}

	return nil, 0, errors.New("странно, мы не должны были здесь оказаться")
}

// Обновлений начислений и статусов начислений по заказам
func UpdateAccurals(storage *storage.Database, accrualSystemAddress string) error {
	// получаем список всех заказов со статусами NEW, REGISTERED, PROCESSING
	orders, err := storage.GetOrdersForUpdate()
	if err != nil {
		return err
	}

	sleep := 1 * time.Nanosecond
	// проходим в цикле по списку и получаем из удаленного сервиса обновления
	for _, order := range *orders {
		time.Sleep(sleep)

		accural, retryAfter, err := getOrderAccrual(accrualSystemAddress, order.Number)

		if err != nil && retryAfter > 0 {
			fmt.Println("Retry After: " + fmt.Sprint(retryAfter))
			sleep = time.Duration(retryAfter) * time.Second
			continue
		}

		if err != nil {
			return err
		}

		// обновляем данные по заказу в orders
		fmt.Println(accural)
		err = storage.UpdateOrder(accural.Order, accural.Status, accural.Accrual)
		if err != nil {
			return err
		}
		sleep = 1 * time.Nanosecond
	}

	return nil
}
