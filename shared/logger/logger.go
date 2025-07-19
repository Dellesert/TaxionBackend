package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Logger wraps logrus with additional functionality
type Logger struct {
	*logrus.Logger
}

// Config holds logger configuration
type Config struct {
	Level       string // debug, info, warn, error
	Format      string // json, text
	Environment string // development, production
}

// DefaultConfig returns default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:       "info",
		Format:      "text",
		Environment: "development",
	}
}

// New creates a new logger instance
func New(config *Config) *Logger {
	log := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Set output format
	if config.Format == "json" || config.Environment == "production" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// Set output to stdout
	log.SetOutput(os.Stdout)

	return &Logger{log}
}

// WithFields adds structured fields to log entry
func (l *Logger) WithFields(fields map[string]interface{}) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields(fields))
}

// WithField adds a single field to log entry
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

// WithError adds an error field to log entry
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}

// Debug logs a debug message
func (l *Logger) Debug(args ...interface{}) {
	l.Logger.Debug(args...)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Logger.Debugf(format, args...)
}

// Info logs an info message
func (l *Logger) Info(args ...interface{}) {
	l.Logger.Info(args...)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Logger.Infof(format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(args ...interface{}) {
	l.Logger.Warn(args...)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Logger.Warnf(format, args...)
}

// Error logs an error message
func (l *Logger) Error(args ...interface{}) {
	l.Logger.Error(args...)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Logger.Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(args ...interface{}) {
	l.Logger.Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Logger.Fatalf(format, args...)
}

// Global logger instance
var defaultLogger *Logger

// init initializes the default logger
func init() {
	defaultLogger = New(DefaultConfig())
}

// Global logging functions using the default logger

// Debug logs a debug message using default logger
func Debug(args ...interface{}) {
	defaultLogger.Debug(args...)
}

// Debugf logs a formatted debug message using default logger
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

// Info logs an info message using default logger
func Info(args ...interface{}) {
	defaultLogger.Info(args...)
}

// Infof logs a formatted info message using default logger
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Warn logs a warning message using default logger
func Warn(args ...interface{}) {
	defaultLogger.Warn(args...)
}

// Warnf logs a formatted warning message using default logger
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

// Error logs an error message using default logger
func Error(args ...interface{}) {
	defaultLogger.Error(args...)
}

// Errorf logs a formatted error message using default logger
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// WithFields adds structured fields using default logger
func WithFields(fields map[string]interface{}) *logrus.Entry {
	return defaultLogger.WithFields(fields)
}

// WithField adds a single field using default logger
func WithField(key string, value interface{}) *logrus.Entry {
	return defaultLogger.WithField(key, value)
}

// WithError adds an error field using default logger
func WithError(err error) *logrus.Entry {
	return defaultLogger.WithError(err)
}
