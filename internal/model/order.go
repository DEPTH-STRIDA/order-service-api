package model

type Order struct {
	ID         uint    `json:"id"`
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Quantity   float32 `json:"quantity"`
	Price      float32 `json:"price"`
	Status     string  `json:"status"`
	TimeStamp  string  `json:"timestamp"`
	BinanceID  int64   `json:"binance_id"`
	StrategyID int64   `json:"strategy_id"`

	// Что делать с ордером: place,cancel,edit
	Action string `json:"action"`
	// Статус заявки, успешно отправлено или нет.
	OrderApiStatus string `json:"order_api_status"`
}
