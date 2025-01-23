package main

import (
	"app/internal/biance"
	"app/internal/kafka"
	"app/internal/logger"
	"app/internal/model"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LoggerLevel        int    `envconfig:"LOGGER_LEVEL"`
	BianceApiPublicKey string `envconfig:"BIANCE_API_PUBLIC_KEY"`
	BianceApiSecretKey string `envconfig:"BIANCE_API_SECTER_KEY"`
	NewOrdersTopic     string `envconfig:"NEW_ORDERS_TOPIC"`
	ReadyOrdersTopic   string `envconfig:"READY_ORDERS_TOPIC"`
	KafkaUrl           string `envconfig:"KAFKA_URL"`
	BianceUrl          string `envconfig:"BIANCE_URL"`

	BianceRequestPauseMilli int `envconfig:"Biance_Request_Pause_Mili"`
}

func main() {
	var err error

	// Загружаем .env файл
	err = godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла: ", err)
	}

	// Создаем и заполняем конфигурацию
	var config Config
	err = envconfig.Process("", &config)
	if err != nil {
		log.Fatal("Ошибка чтения конфигурации из переменных окружения: ", err)
	}

	fmt.Printf("Загружена конфигурация: %+v\n", config)

	// Создание логгера
	logger.Log, err = logger.NewLogger(config.LoggerLevel)
	if err != nil {
		panic(err)
	}

	newOrders := make(chan model.Order)
	kafka, err := kafka.NewKafkaManager(config.NewOrdersTopic, config.ReadyOrdersTopic, config.KafkaUrl, newOrders)
	handlerError(err)

	// Чтение из канала новых сообщений кафки
	go kafka.StartReadingKafka()

	bianceManager, err := biance.NewBianceManager(config.BianceUrl, config.BianceApiPublicKey, config.BianceApiSecretKey, time.Duration(config.BianceRequestPauseMilli)*time.Millisecond)
	if err != nil {
		handlerError(err)
	}

	bianceManager.CheckAndTestLimits()
}

func handlerError(err error) {
	if err != nil {
		logger.Log.Error(err)
		panic(err)
	}
}
