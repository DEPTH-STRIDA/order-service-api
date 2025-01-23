package biance

import (
	"app/internal/logger"
	"app/internal/model"
	"app/internal/request"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/adshao/go-binance/v2"
)

const (
	PlaceOrder  = "place_order"
	EditOrder   = "edit_order"
	CancelOrder = "cancel_orders"
)

type BianceManager struct {
	url       string
	apiKey    string
	secretKey string
	client    *binance.Client
	requester *request.RequestHandler
}

type loggingRoundTripper struct {
	next http.RoundTripper
}

func NewBianceManager(url, apiKey, secretKey string, pause time.Duration) (*BianceManager, error) {

	httpClient := &http.Client{
		Timeout: time.Second * 10,
		Transport: &loggingRoundTripper{
			next: http.DefaultTransport,
		},
	}
	client := binance.NewClient(apiKey, secretKey)
	client.BaseURL = url
	client.HTTPClient = httpClient

	re, err := request.NewRequestHandler(10)
	if err != nil {
		return nil, err
	}
	re.ProcessRequests(pause)

	bianceManager := BianceManager{
		url:       url,
		apiKey:    apiKey,
		secretKey: secretKey,
		client:    client,
		requester: re,
	}
	return &bianceManager, nil
}

func (l loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody, _ := httputil.DumpRequestOut(req, true)
	logger.Log.Info("Отправка запроса:\n", string(reqBody), "\n")

	resp, err := l.next.RoundTrip(req)
	if err != nil {
		logger.Log.Info("Ошибка при отправке запроса: ", err)
		return resp, err
	}

	// respBody, _ := httputil.DumpResponse(resp, true)
	// logger.Log.Info("Получен ответ:\n", string(respBody), "\n")

	return resp, err
}

func (bm *BianceManager) checkAPIAvailability() {
	resp, err := http.Get(bm.url + "/api/v3/ping")
	if err != nil {
		logger.Log.Error("Ошибка при проверке доступности API: ", err, "\n")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		logger.Log.Error("API доступен")
	} else {
		logger.Log.Error("API недоступен. Код статуса: ", resp.StatusCode, "\n")
	}
}

func (bm *BianceManager) checkSystemStatus() {
	logger.Log.Info("Отправка запроса на проверку статуса системы")

	resp, err := http.Get(bm.url + "/api/v3/system/status")
	if err != nil {
		logger.Log.Error("Ошибка при проверке статуса системы: ", err, "\n")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Log.Error("Ошибка при чтении ответа: ", err, "\n")
		return
	}
	logger.Log.Info("Статус системы: ", string(body), "\n")
}

func (bm *BianceManager) checkConnection() {
	logger.Log.Info("Отправка запроса на проверку подключения")

	account, err := bm.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		logger.Log.Error("Ошибка при проверке подключения: ", err, "\n")
		return
	}
	log.Println("Подключение успешно. Баланс аккаунта получен.")
	for _, balance := range account.Balances {
		if balance.Free != "0.00000000" || balance.Locked != "0.00000000" {
			logger.Log.Info(fmt.Sprintf("Актив: %s, Свободно: %s, Заблокировано: %s\n", balance.Asset, balance.Free, balance.Locked))
		}
	}
}

func (bm *BianceManager) ProcessOrders(newOrders chan model.Order) {
	for v := range newOrders {
		bm.switchOrder(v)
	}
}

func (bm *BianceManager) switchOrder(order model.Order) {
	var err error
	var orderId int64

	switch order.Action {
	case PlaceOrder:
		// Выполняем синхронный запрос
		bm.requester.SyncHandleRequest(func() error {
			orderId, err = bm.placeOrder(order)
			return err
		})

	case EditOrder:
		//
		bm.requester.SyncHandleRequest(func() error {
			orderId, err = bm.editOrder(order)
			return err
		})

	case CancelOrder:
		//

		bm.requester.SyncHandleRequest(func() error {
			err = bm.cancelOrder(order)
			return err
		})

	default:
		logger.Log.Info("Неизвестное действие: ", order.Action, "\n")
		return
	}

	if err != nil {
		logger.Log.Error(fmt.Sprintf("Ошибка при выполнении действия %s: %v\n", order.Action, err))
	} else {
		if order.Action != CancelOrder {
			logger.Log.Info(fmt.Sprintf("Действие %s выполнено успешно. Новый ID ордера: %d\n", order.Action, orderId))
		} else {
			logger.Log.Info("Ордер успешно отменен\n")
		}
	}
}

func (bm *BianceManager) placeOrder(order model.Order) (int64, error) {
	orderSide := binance.SideType(order.Side)
	// orderType := binance.OrderTypeLimit

	newOrder, err := bm.client.NewCreateOrderService().Symbol(order.Symbol).Side(orderSide).Quantity(fmt.Sprintf("%f", order.Quantity)).Price(fmt.Sprintf("%f", order.Price)).Do(context.Background())

	// Type(orderType).
	// TimeInForce(binance.TimeInForceTypeGTC).

	if err != nil {
		return 0, fmt.Errorf("ошибка при размещении ордера: %v", err)
	}

	return newOrder.OrderID, nil
}

func (bm *BianceManager) cancelOrder(order model.Order) error {

	_, err := bm.client.NewCancelOrderService().
		Symbol(order.Symbol).
		OrderID(order.BinanceID).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("ошибка при отмене ордера: %v", err)
	}

	return nil
}

func (bm *BianceManager) editOrder(order model.Order) (int64, error) {
	logger.Log.Info(fmt.Sprintf("Попытка обновления ордера: Symbol=%s, OrderID=%d, NewQuantity=%f, NewPrice=%f\n",
		order.Symbol, order.BinanceID, order.Quantity, order.Price))

	// Сначала отменяем существующий ордер
	err := bm.cancelOrder(order)
	if err != nil {
		return 0, fmt.Errorf("ошибка при отмене старого ордера: %v", err)
	}

	// Создаем новый ордер с обновленными параметрами
	newOrderID, err := bm.placeOrder(order)
	if err != nil {
		return 0, fmt.Errorf("ошибка при создании нового ордера: %v", err)
	}

	// log.Printf("Ордер успешно обновлен. Старый OrderID: %d, Новый OrderID: %d\n", order.BinanceID, newOrderID)

	return newOrderID, nil
}

func (bm *BianceManager) CheckAndTestLimits() {
	fmt.Println("Проверка и тестирование лимитов Binance API")

	// 1. Проверка общей информации об ограничениях
	fmt.Println("\n1. Проверка общей информации:")
	exchangeInfo, err := bm.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		fmt.Printf("Ошибка при получении exchangeInfo: %v\n", err)
	} else {
		fmt.Printf("Количество торговых пар: %d\n", len(exchangeInfo.Symbols))
		// Вывод лимитов для первой торговой пары
		if len(exchangeInfo.Symbols) > 0 {
			symbol := exchangeInfo.Symbols[0]
			fmt.Printf("Лимиты для %s:\n", symbol.Symbol)
			for _, filter := range symbol.Filters {
				fmt.Printf("  %s: %+v\n", filter["filterType"], filter)
			}
		}
	}

	// 2. Тест лимита запросов
	fmt.Println("\n2. Тест лимита запросов:")
	startTime := time.Now()
	requestCount := 0
	for time.Since(startTime) < time.Minute {
		err := bm.client.NewPingService().Do(context.Background())
		if err != nil {
			fmt.Printf("Ошибка при отправке ping: %v\n", err)
			break
		}
		requestCount++
		if requestCount%100 == 0 {
			fmt.Printf("Отправлено запросов: %d\n", requestCount)
		}
		time.Sleep(time.Millisecond * 50) // Небольшая задержка между запросами
	}
	fmt.Printf("Всего отправлено запросов за минуту: %d\n", requestCount)

	// 4. Тест создания и отмены ордера
	fmt.Println("\n4. Тест создания и отмены ордера:")
	testOrder := model.Order{
		Symbol: "BTCUSDT",
		Side:   "BUY",
		// Type:     "LIMIT",
		Quantity: 0.001,
		Price:    10000, // Установите цену значительно ниже рыночной
	}
	orderId, err := bm.placeOrder(testOrder)
	if err != nil {
		fmt.Printf("Ошибка при создании тестового ордера: %v\n", err)
	} else {
		fmt.Printf("Тестовый ордер создан, ID: %d\n", orderId)
		err = bm.cancelOrder(model.Order{Symbol: "BTCUSDT", BinanceID: orderId})
		if err != nil {
			fmt.Printf("Ошибка при отмене тестового ордера: %v\n", err)
		} else {
			fmt.Println("Тестовый ордер успешно отменен")
		}
	}

	fmt.Println("\nПроверка и тестирование завершены")
}
