package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
)

const (
	apiTestnetBaseURL = "https://testnet.binance.vision"
	apiKey            = "267m3WzuyQnk1ey9S5id5mgZgFmyK9RtIJEZJTOibHWbsDy48LXjSLFe5H7nWlVv"
	secretKey         = "3CKIT7vpv6KodAOJWSB39AtSMYw5qQeaqFiIKUgayuTBDsuwOvnmCl6mUNTWWT9W"
)

func main() {
	log.Println("Начало выполнения программы")

	httpClient := &http.Client{
		Timeout: time.Second * 10,
		Transport: &loggingRoundTripper{
			next: http.DefaultTransport,
		},
	}

	log.Println("Создание клиента Binance")
	client := binance.NewClient(apiKey, secretKey)
	client.BaseURL = apiTestnetBaseURL
	client.HTTPClient = httpClient

	log.Println("Проверка доступности API")
	checkAPIAvailability()

	log.Println("Проверка подключения")
	checkConnection(client)

	log.Println("Проверка статуса системы")
	checkSystemStatus()

	messages := []string{
		"place_order,BTCUSDT,BUY,0.001,50000",
		"cancel_order,BTCUSDT,1234567",
		"edit_order,BTCUSDT,7654321,0.002,51000",
	}

	for _, msg := range messages {
		log.Printf("Обработка сообщения: %s\n", msg)
		processMessage(client, msg)
	}

	log.Println("Завершение выполнения программы")
}

type loggingRoundTripper struct {
	next http.RoundTripper
}

func (l loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody, _ := httputil.DumpRequestOut(req, true)
	log.Printf("Отправка запроса:\n%s\n", string(reqBody))

	resp, err := l.next.RoundTrip(req)
	if err != nil {
		log.Printf("Ошибка при отправке запроса: %v\n", err)
		return resp, err
	}

	respBody, _ := httputil.DumpResponse(resp, true)
	log.Printf("Получен ответ:\n%s\n", string(respBody))

	return resp, err
}

func checkAPIAvailability() {
	resp, err := http.Get(apiTestnetBaseURL + "/api/v3/ping")
	if err != nil {
		log.Printf("Ошибка при проверке доступности API: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Println("API доступен")
	} else {
		log.Printf("API недоступен. Код статуса: %d\n", resp.StatusCode)
	}
}

func checkSystemStatus() {
	log.Println("Отправка запроса на проверку статуса системы")
	resp, err := http.Get(apiTestnetBaseURL + "/api/v3/system/status")
	if err != nil {
		log.Printf("Ошибка при проверке статуса системы: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Ошибка при чтении ответа: %v\n", err)
		return
	}

	log.Printf("Статус системы: %s\n", string(body))
}

func checkConnection(client *binance.Client) {
	log.Println("Отправка запроса на проверку подключения")
	account, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		log.Printf("Ошибка при проверке подключения: %v\n", err)
		return
	}
	log.Println("Подключение успешно. Баланс аккаунта получен.")
	for _, balance := range account.Balances {
		if balance.Free != "0.00000000" || balance.Locked != "0.00000000" {
			log.Printf("Актив: %s, Свободно: %s, Заблокировано: %s\n", balance.Asset, balance.Free, balance.Locked)
		}
	}
}

func processMessage(client *binance.Client, msg string) {
	parts := splitMessage(msg)
	if len(parts) < 2 {
		log.Printf("Неверный формат сообщения: %s\n", msg)
		return
	}

	action := parts[0]
	symbol := parts[1]

	switch action {
	case "place_order":
		if len(parts) != 5 {
			log.Printf("Неверное количество параметров для размещения ордера: %s\n", msg)
			return
		}
		side, quantity, price := parts[2], parts[3], parts[4]
		log.Printf("Попытка размещения ордера: Symbol=%s, Side=%s, Quantity=%s, Price=%s\n", symbol, side, quantity, price)
		orderID, err := placeOrder(client, symbol, side, quantity, price)
		if err != nil {
			log.Printf("Ошибка при размещении ордера: %v\n", err)
		} else {
			log.Printf("Ордер размещен успешно. ID: %d\n", orderID)
		}

	case "cancel_order":
		if len(parts) != 3 {
			log.Printf("Неверное количество параметров для отмены ордера: %s\n", msg)
			return
		}
		orderID := parts[2]
		log.Printf("Попытка отмены ордера: Symbol=%s, OrderID=%s\n", symbol, orderID)
		err := cancelOrder(client, symbol, orderID)
		if err != nil {
			log.Printf("Ошибка при отмене ордера: %v\n", err)
		} else {
			log.Printf("Ордер %s успешно отменен\n", orderID)
		}

	case "edit_order":
		if len(parts) != 5 {
			log.Printf("Неверное количество параметров для редактирования ордера: %s\n", msg)
			return
		}
		orderID, newQuantity, newPrice := parts[2], parts[3], parts[4]
		log.Printf("Попытка редактирования ордера: Symbol=%s, OrderID=%s, NewQuantity=%s, NewPrice=%s\n", symbol, orderID, newQuantity, newPrice)
		newOrderID, err := editOrder(client, symbol, orderID, newQuantity, newPrice)
		if err != nil {
			log.Printf("Ошибка при редактировании ордера: %v\n", err)
		} else {
			log.Printf("Ордер успешно отредактирован. Новый ID: %d\n", newOrderID)
		}

	default:
		log.Printf("Неизвестное действие: %s\n", action)
	}
}

func placeOrder(client *binance.Client, symbol, side, quantity, price string) (int64, error) {
	orderSide := binance.SideType(side)
	orderType := binance.OrderTypeLimit

	order, err := client.NewCreateOrderService().
		Symbol(symbol).
		Side(orderSide).
		Type(orderType).
		TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(quantity).
		Price(price).
		Do(context.Background())

	if err != nil {
		return 0, fmt.Errorf("ошибка при размещении ордера: %v", err)
	}

	return order.OrderID, nil
}

func cancelOrder(client *binance.Client, symbol, orderID string) error {
	orderIDInt, _ := strconv.ParseInt(orderID, 10, 64)

	_, err := client.NewCancelOrderService().
		Symbol(symbol).
		OrderID(orderIDInt).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("ошибка при отмене ордера: %v", err)
	}

	return nil
}

func editOrder(client *binance.Client, symbol, orderID, newQuantity, newPrice string) (int64, error) {
	err := cancelOrder(client, symbol, orderID)
	if err != nil {
		return 0, fmt.Errorf("ошибка при отмене старого ордера: %v", err)
	}

	return placeOrder(client, symbol, "BUY", newQuantity, newPrice)
}

func splitMessage(msg string) []string {
	var result []string
	var current []rune
	inQuotes := false

	for _, char := range msg {
		if char == ',' && !inQuotes {
			result = append(result, string(current))
			current = []rune{}
		} else if char == '"' {
			inQuotes = !inQuotes
		} else {
			current = append(current, char)
		}
	}
	result = append(result, string(current))

	return result
}
