package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	topic         = "test-topic"
	brokerAddress = "localhost:9092"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := createTopic(ctx); err != nil {
		log.Fatalf("Ошибка при создании топика: %v", err)
	}

	go produceMessages(ctx)
	go consumeMessages(ctx)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	<-sigchan

	fmt.Println("Завершение программы...")
}

func createTopic(ctx context.Context) error {
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

	log.Printf("Топик '%s' успешно создан или уже существует", topic)
	return nil
}

func produceMessages(ctx context.Context) {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{brokerAddress},
		Topic:   topic,
	})
	defer writer.Close()

	for i := 0; ; i++ {
		message := fmt.Sprintf("Сообщение %d", i)
		err := writer.WriteMessages(ctx, kafka.Message{
			Value: []byte(message),
		})
		if err != nil {
			log.Printf("Ошибка при отправке сообщения: %v", err)
		} else {
			log.Printf("Отправлено: %s", message)
		}
		time.Sleep(time.Second * 5) // Отправка сообщения каждые 5 секунд
	}
}

func consumeMessages(ctx context.Context) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{brokerAddress},
		Topic:   topic,
		GroupID: "test-group",
	})
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			log.Printf("Ошибка при чтении сообщения: %v", err)
			continue
		}

		log.Printf("Получено сообщение: %s (partition: %d, offset: %d)", string(msg.Value), msg.Partition, msg.Offset)

		// Имитация отправки запроса к внешнему API
		if err := simulateExternalAPI(string(msg.Value)); err != nil {
			log.Printf("Ошибка при обработке сообщения: %v", err)
			continue
		}

		// Фиксация смещения только после успешной "отправки запроса"
		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("Ошибка при фиксации смещения: %v", err)
		} else {
			log.Printf("Смещение зафиксировано для partition: %d, offset: %d", msg.Partition, msg.Offset)
		}
	}
}

func simulateExternalAPI(data string) error {
	log.Printf("Начало обработки сообщения: %s", data)

	// Имитация времени обработки запроса
	processingTime := time.Duration(rand.Intn(3)+1) * time.Second
	time.Sleep(processingTime)

	// Имитация случайной ошибки (10% шанс)
	if rand.Float32() < 0.1 {
		return fmt.Errorf("симуляция ошибки API для сообщения: %s", data)
	}

	log.Printf("Сообщение успешно обработано за %v: %s", processingTime, data)
	return nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
