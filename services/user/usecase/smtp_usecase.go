package usecase

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/logger"
)

// SMTPUsecase defines interface for SMTP settings use cases
type SMTPUsecase interface {
	GetSettings() (*models.SMTPSettingsResponse, error)
	UpdateSettings(req *models.UpdateSMTPSettingsRequest, updatedBy uint) (*models.SMTPSettingsResponse, error)
	TestConnection(req *models.TestSMTPConnectionRequest) (*models.TestSMTPConnectionResponse, error)
}

// smtpUsecase implements SMTPUsecase
type smtpUsecase struct {
	smtpRepo repository.SMTPRepository
}

// NewSMTPUsecase creates a new SMTP usecase
func NewSMTPUsecase(smtpRepo repository.SMTPRepository) SMTPUsecase {
	return &smtpUsecase{
		smtpRepo: smtpRepo,
	}
}

// GetSettings retrieves current SMTP settings
func (u *smtpUsecase) GetSettings() (*models.SMTPSettingsResponse, error) {
	settings, err := u.smtpRepo.GetSettings()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to get SMTP settings")
		return nil, fmt.Errorf("failed to get SMTP settings: %w", err)
	}

	if settings == nil {
		// Return nil if no settings exist yet
		return nil, nil
	}

	return settings.ToResponse(), nil
}

// UpdateSettings updates SMTP settings
func (u *smtpUsecase) UpdateSettings(req *models.UpdateSMTPSettingsRequest, updatedBy uint) (*models.SMTPSettingsResponse, error) {
	// Get existing settings
	existingSettings, err := u.smtpRepo.GetSettings()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to get existing SMTP settings")
		return nil, fmt.Errorf("failed to get existing SMTP settings: %w", err)
	}

	// Check if settings exist
	if existingSettings == nil {
		// Create new settings
		newSettings := &models.SMTPSettings{
			Host:              req.Host,
			Port:              req.Port,
			Username:          req.Username,
			Password:          *req.Password, // Password is required for creation
			FromEmail:         req.FromEmail,
			FromName:          req.FromName,
			UseTLS:            req.UseTLS,
			UseSSL:            req.UseSSL,
			TimeoutSeconds:    req.TimeoutSeconds,
			MaxRetries:        req.MaxRetries,
			RetryDelaySeconds: req.RetryDelaySeconds,
			PoolSize:          req.PoolSize,
			RateLimitRPS:      req.RateLimitRPS,
			UpdatedBy:         updatedBy,
		}

		if err := u.smtpRepo.CreateSettings(newSettings); err != nil {
			logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to create SMTP settings")
			return nil, fmt.Errorf("failed to create SMTP settings: %w", err)
		}

		logger.WithFields(map[string]interface{}{
			"updated_by": updatedBy,
			"host":       req.Host,
		}).Info("SMTP settings created successfully")

		return newSettings.ToResponse(), nil
	}

	// Update existing settings
	existingSettings.Host = req.Host
	existingSettings.Port = req.Port
	existingSettings.Username = req.Username
	existingSettings.FromEmail = req.FromEmail
	existingSettings.FromName = req.FromName
	existingSettings.UseTLS = req.UseTLS
	existingSettings.UseSSL = req.UseSSL
	existingSettings.TimeoutSeconds = req.TimeoutSeconds
	existingSettings.MaxRetries = req.MaxRetries
	existingSettings.RetryDelaySeconds = req.RetryDelaySeconds
	existingSettings.PoolSize = req.PoolSize
	existingSettings.RateLimitRPS = req.RateLimitRPS
	existingSettings.UpdatedBy = updatedBy

	// Only update password if provided
	if req.Password != nil && *req.Password != "" {
		existingSettings.Password = *req.Password
	}

	if err := u.smtpRepo.UpdateSettings(existingSettings); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"id":    existingSettings.ID,
		}).Error("Failed to update SMTP settings")
		return nil, fmt.Errorf("failed to update SMTP settings: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"updated_by": updatedBy,
		"host":       req.Host,
		"id":         existingSettings.ID,
	}).Info("SMTP settings updated successfully")

	return existingSettings.ToResponse(), nil
}

// TestConnection tests SMTP connection with provided settings
func (u *smtpUsecase) TestConnection(req *models.TestSMTPConnectionRequest) (*models.TestSMTPConnectionResponse, error) {
	logger.WithFields(map[string]interface{}{
		"host":       req.Host,
		"port":       req.Port,
		"use_tls":    req.UseTLS,
		"use_ssl":    req.UseSSL,
		"test_email": req.TestEmail,
	}).Info("Testing SMTP connection")

	// Test the connection
	err := u.testSMTPConnection(req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"host":  req.Host,
			"port":  req.Port,
		}).Error("SMTP connection test failed")

		return &models.TestSMTPConnectionResponse{
			Success: false,
			Message: "Не удалось подключиться к SMTP серверу",
			Error:   err.Error(),
		}, nil
	}

	logger.WithFields(map[string]interface{}{
		"host": req.Host,
		"port": req.Port,
	}).Info("SMTP connection test successful")

	return &models.TestSMTPConnectionResponse{
		Success: true,
		Message: "Подключение к SMTP серверу выполнено успешно",
	}, nil
}

// testSMTPConnection performs actual SMTP connection test
func (u *smtpUsecase) testSMTPConnection(req *models.TestSMTPConnectionRequest) error {
	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)

	// Handle SSL/TLS connection
	if req.UseSSL {
		return u.testSMTPWithSSL(addr, req)
	}

	return u.testSMTPWithTLS(addr, req)
}

// testSMTPWithTLS tests SMTP with STARTTLS
func (u *smtpUsecase) testSMTPWithTLS(addr string, req *models.TestSMTPConnectionRequest) error {
	// Set timeout for connection
	timeout := 10 * time.Second

	// Create a channel for the result
	errChan := make(chan error, 1)

	go func() {
		// Connect to server
		client, err := smtp.Dial(addr)
		if err != nil {
			errChan <- fmt.Errorf("failed to connect to SMTP server: %w", err)
			return
		}
		defer client.Close()

		// Start TLS if enabled
		if req.UseTLS {
			tlsConfig := &tls.Config{
				ServerName: req.Host,
			}
			if err := client.StartTLS(tlsConfig); err != nil {
				errChan <- fmt.Errorf("failed to start TLS: %w", err)
				return
			}
		}

		// Authenticate
		auth := smtp.PlainAuth("", req.Username, req.Password, req.Host)
		if err := client.Auth(auth); err != nil {
			errChan <- fmt.Errorf("SMTP authentication failed: %w", err)
			return
		}

		// Try to send a test email
		if err := client.Mail(req.FromEmail); err != nil {
			errChan <- fmt.Errorf("failed to set sender: %w", err)
			return
		}

		if err := client.Rcpt(req.TestEmail); err != nil {
			errChan <- fmt.Errorf("failed to set recipient: %w", err)
			return
		}

		// Don't actually send the email, just close the connection
		errChan <- nil
	}()

	// Wait for result or timeout
	select {
	case err := <-errChan:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("connection test timed out after %v", timeout)
	}
}

// testSMTPWithSSL tests SMTP with SSL/TLS connection
func (u *smtpUsecase) testSMTPWithSSL(addr string, req *models.TestSMTPConnectionRequest) error {
	// Build test message
	message := u.buildTestMessage(req.FromEmail, req.TestEmail)

	// Setup authentication
	auth := smtp.PlainAuth("", req.Username, req.Password, req.Host)

	// Try to send (but we'll just test the connection)
	err := smtp.SendMail(addr, auth, req.FromEmail, []string{req.TestEmail}, []byte(message))
	if err != nil {
		// Check if it's an authentication or connection error
		if strings.Contains(err.Error(), "authentication") {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
		if strings.Contains(err.Error(), "connection") {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		return fmt.Errorf("SMTP test failed: %w", err)
	}

	return nil
}

// buildTestMessage builds a test email message
func (u *smtpUsecase) buildTestMessage(from, to string) string {
	subject := "SMTP Connection Test"
	body := "This is a test email to verify SMTP configuration."

	message := fmt.Sprintf("From: %s\r\n", from)
	message += fmt.Sprintf("To: %s\r\n", to)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "MIME-Version: 1.0\r\n"
	message += "Content-Type: text/plain; charset=utf-8\r\n"
	message += "\r\n"
	message += body

	return message
}
