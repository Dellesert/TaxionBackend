package sentry

import (
	"fmt"
	"os"
	"time"

	"tachyon-messenger/shared/logger"

	"github.com/getsentry/sentry-go"
)

// Init initializes the Sentry SDK for a given service.
// If dsn is empty, Sentry is disabled silently.
func Init(dsn, serviceName string) error {
	if dsn == "" {
		logger.Info("Sentry DSN not set, error tracking disabled")
		return nil
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		ServerName:       serviceName,
		TracesSampleRate: 0.2,
		EnableTracing:    true,
	})
	if err != nil {
		return fmt.Errorf("sentry init failed: %w", err)
	}

	logger.Infof("Sentry initialized for %s (%s)", serviceName, environment)
	return nil
}

// Flush waits for buffered events to be sent before shutdown.
func Flush() {
	sentry.Flush(2 * time.Second)
}
