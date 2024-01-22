package shutdown

import (
	"log"
)

// StandardLogger is a logger that uses the standard log package.
type StandardLogger struct {
	logger *log.Logger
}

// NewStandardLogger creates a logger wrapping a log.Logger instance.
func NewStandardLogger(logger *log.Logger) StandardLogger {
	return StandardLogger{logger: logger}
}

func (d StandardLogger) Info(text string) {
	d.logger.Printf(text)
}

func (d StandardLogger) Error(text string) {
	d.logger.Printf(text)
}
