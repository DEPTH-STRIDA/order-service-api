package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Глобальная переменная для работы с другими пакетами
var Log Logger

// LogLevel определяет уровень логирования
type LogLevel string

const (
	INFO  LogLevel = "INFO"
	ERROR LogLevel = "ERROR"
	WARN  LogLevel = "WARN"
	DEBUG LogLevel = "DEBUG"
)

// LogEntry структура для записи логов в формате JSON
type LogEntry struct {
	Timestamp string   `json:"time"`
	Level     LogLevel `json:"level"`
	Message   string   `json:"message"`
}

// Logger интерфейс для логгеров
type Logger interface {
	Error(args ...interface{})
	Warn(args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
}

// ConsoleLogger логгер для логирования в консоль
type ConsoleLogger struct {
	mu sync.Mutex
}

// NewConsoleLogger конструктор консольного логгера
func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{}
}

func (c *ConsoleLogger) log(level LogLevel, args ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().Format("02-01-2006 15:04:05"),
		Level:     level,
		Message:   fmt.Sprint(args...),
	}
	data, _ := json.Marshal(entry)
	fmt.Println(string(data))
}

func (c *ConsoleLogger) Warn(args ...interface{})  { c.log(WARN, args...) }
func (c *ConsoleLogger) Error(args ...interface{}) { c.log(ERROR, args...) }
func (c *ConsoleLogger) Info(args ...interface{})  { c.log(INFO, args...) }
func (c *ConsoleLogger) Debug(args ...interface{}) { c.log(DEBUG, args...) }

// FileLogger логгер для логирования в файл с ротацией
type FileLogger struct {
	mu     sync.Mutex
	writer *lumberjack.Logger
}

// NewFileLogger конструктор файлового логгера
func NewFileLogger(basePath string) (*FileLogger, error) {
	dir := filepath.Dir(basePath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	writer := &lumberjack.Logger{
		Filename:   filepath.Join(dir, "log.log"),
		MaxSize:    10, // мегабайты
		MaxBackups: 5,
		MaxAge:     7, // дни
		Compress:   true,
	}

	return &FileLogger{
		writer: writer,
	}, nil
}

func (f *FileLogger) log(level LogLevel, args ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().Format("02-01-2006 15:04:05"),
		Level:     level,
		Message:   fmt.Sprint(args...),
	}

	data, _ := json.Marshal(entry)
	f.writer.Write(append(data, '\n'))
}

func (f *FileLogger) Warn(args ...interface{})  { f.log(WARN, args...) }
func (f *FileLogger) Error(args ...interface{}) { f.log(ERROR, args...) }
func (f *FileLogger) Info(args ...interface{})  { f.log(INFO, args...) }
func (f *FileLogger) Debug(args ...interface{}) { f.log(DEBUG, args...) }

// CombinedLogger логгер для логирования и в консоль, и в файл
type CombinedLogger struct {
	fileLogger    *FileLogger
	consoleLogger *ConsoleLogger
}

func NewCombinedLogger(path string) (*CombinedLogger, error) {
	fileLogger, err := NewFileLogger(path)
	if err != nil {
		return nil, err
	}
	consoleLogger := NewConsoleLogger()
	return &CombinedLogger{
		fileLogger:    fileLogger,
		consoleLogger: consoleLogger,
	}, nil
}

func (c *CombinedLogger) Warn(args ...interface{}) {
	c.fileLogger.Warn(args...)
	c.consoleLogger.Warn(args...)
}

func (c *CombinedLogger) Error(args ...interface{}) {
	c.fileLogger.Error(args...)
	c.consoleLogger.Error(args...)
}

func (c *CombinedLogger) Info(args ...interface{}) {
	c.fileLogger.Info(args...)
	c.consoleLogger.Info(args...)
}

func (c *CombinedLogger) Debug(args ...interface{}) {
	c.fileLogger.Debug(args...)
	c.consoleLogger.Debug(args...)
}

func (level LogLevel) String() string {
	return string(level)
}
