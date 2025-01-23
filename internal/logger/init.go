package logger

import (
	"fmt"
)

func NewLogger(loggerTypeInt int) (Logger, error) {

	switch loggerTypeInt {
	case 0:
		return NewConsoleLogger(), nil
	case 1:
		logger, err := NewFileLogger("/logs/")
		return logger, err
	case 2:
		logger, err := NewCombinedLogger("/logs/")
		return logger, err
	default:
		return nil, fmt.Errorf("wrong n - %d. N can be only: 0,1,2", loggerTypeInt)
	}
}
