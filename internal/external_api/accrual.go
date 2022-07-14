package externalapi

type AccuralType struct {
	Order   string `json:"order"`             // номер заказа
	Status  string `json:"status"`            // статус расчёта начисления
	Accrual int    `json:"accrual,omitempty"` // рассчитанные баллы к начислению, при отсутствии начисления — поле отсутствует в ответе
}
