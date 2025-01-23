package kafka

import (
	"app/internal/logger"
	"app/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

// Небольшая надстройка над структурой для работы с ордерами из кафки
type OrderKafka struct {
	new_orders chan model.Order
	ctx        context.Context
	writer     *kafka.Writer
	reader     *kafka.Reader
}

func NewKafkaManager(newOrderTopic, readyOrderTopic, brokerAddress string, new_orders chan model.Order) (*OrderKafka, error) {
	orderKafka := OrderKafka{
		ctx: context.Background(),
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{brokerAddress},
			Topic:   newOrderTopic,
		}),
		writer: kafka.NewWriter(kafka.WriterConfig{
			Brokers: []string{brokerAddress},
			Topic:   readyOrderTopic,
		}),
	}

	if err := orderKafka.createTopic(orderKafka.ctx, newOrderTopic, brokerAddress); err != nil {
		logger.Log.Error("ошибка при создании топика: %v" + err.Error())
		return nil, fmt.Errorf("ошибка при создании топика: %v", err)
	}
	if err := orderKafka.createTopic(orderKafka.ctx, readyOrderTopic, brokerAddress); err != nil {
		logger.Log.Error("ошибка при создании топика: %v" + err.Error())
		return nil, fmt.Errorf("ошибка при создании топика: %v", err)
	}

	// sigchan := make(chan os.Signal, 1)
	// signal.Notify(sigchan, os.Interrupt)
	// <-sigchan
	// fmt.Println("Завершение программы...")

	return &orderKafka, nil
}

// Close закрывает все каналы и контексты
func (k *OrderKafka) Close() {
	close(k.new_orders)
	k.writer.Close()
	k.reader.Close()
}

func (k *OrderKafka) createTopic(ctx context.Context, topic, brokerAddress string) error {
	conn, err := kafka.DialContext(ctx, "tcp", brokerAddress)
	if err != nil {
		return fmt.Errorf("ошибка подключения к Kafka: %v", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("ошибка получения контроллера: %v", err)
	}
	controllerConn, err := kafka.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("ошибка подключения к контроллеру: %v", err)
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("ошибка создания топика: %v", err)
	}

	logger.Log.Info("Топик '%s' успешно создан или уже существует: ", topic)
	return nil
}

// sendReadyOrders отправляет ордер в кафку в топик готовых ордеров
func (k *OrderKafka) sendReadyOrders(ctx context.Context, order model.Order) error {
	orderJSON, err := json.Marshal(order)
	if err != nil {
		return err
	}

	err = k.writer.WriteMessages(ctx, kafka.Message{
		Value: []byte(orderJSON),
	})
	if err != nil {
		log.Printf("Ошибка при отправке сообщения: %v", err)
	} else {
		log.Printf("Отправлено: %s", orderJSON)
	}
	return nil
}

// StartReadingKafka читает сообщения из кафки и кладет их в канал
func (k *OrderKafka) StartReadingKafka() {

	for {
		msg, err := k.reader.FetchMessage(k.ctx)
		if err != nil {
			logger.Log.Info("Ошибка при чтении сообщения: ", err)
			continue
		}

		logger.Log.Info(fmt.Sprintf("Получено сообщение: %s (partition: %d, offset: %d)", string(msg.Value), msg.Partition, msg.Offset))

		var order model.Order
		err = json.Unmarshal(msg.Value, &order)
		if err != nil {
			logger.Log.Error("Ошибка при анмаршалинге значения смещения: ", err.Error())
		}

		k.new_orders <- order

		// Фиксация смещения только после успешной "отправки запроса"
		if err := k.reader.CommitMessages(k.ctx, msg); err != nil {
			logger.Log.Error("Ошибка при фиксации смещения: ", err.Error())
		} else {
			logger.Log.Error(fmt.Sprintf("Смещение зафиксировано для partition: %d, offset: %d", msg.Partition, msg.Offset))
		}
	}
}
