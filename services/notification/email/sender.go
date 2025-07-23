// File: services/notification/email/sender.go
package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/shared/logger"
)

// EmailSender defines the interface for sending emails
type EmailSender interface {
	SendEmail(req *SendEmailRequest) error
	SendTemplatedEmail(req *TemplatedEmailRequest) error
	SendBulkEmail(req *BulkEmailRequest) error
	ValidateConfig() error
}

// SendEmailRequest represents a simple email sending request
type SendEmailRequest struct {
	To          []string                    `json:"to" validate:"required,min=1,dive,email"`
	CC          []string                    `json:"cc,omitempty" validate:"omitempty,dive,email"`
	BCC         []string                    `json:"bcc,omitempty" validate:"omitempty,dive,email"`
	Subject     string                      `json:"subject" validate:"required,min=1,max=255"`
	HTMLBody    string                      `json:"html_body,omitempty"`
	TextBody    string                      `json:"text_body,omitempty"`
	Attachments []string                    `json:"attachments,omitempty"` // File paths
	Priority    models.NotificationPriority `json:"priority,omitempty"`
}

// TemplatedEmailRequest represents an email request using template
type TemplatedEmailRequest struct {
	To           []string                    `json:"to" validate:"required,min=1,dive,email"`
	CC           []string                    `json:"cc,omitempty" validate:"omitempty,dive,email"`
	BCC          []string                    `json:"bcc,omitempty" validate:"omitempty,dive,email"`
	TemplateName string                      `json:"template_name" validate:"required"`
	Variables    map[string]interface{}      `json:"variables,omitempty"`
	Priority     models.NotificationPriority `json:"priority,omitempty"`
}

// BulkEmailRequest represents a bulk email sending request
type BulkEmailRequest struct {
	Recipients []BulkRecipient             `json:"recipients" validate:"required,min=1,dive"`
	Subject    string                      `json:"subject" validate:"required,min=1,max=255"`
	HTMLBody   string                      `json:"html_body,omitempty"`
	TextBody   string                      `json:"text_body,omitempty"`
	Priority   models.NotificationPriority `json:"priority,omitempty"`
}

// BulkRecipient represents a single recipient in bulk email
type BulkRecipient struct {
	Email     string                 `json:"email" validate:"required,email"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// SMTPConfig holds SMTP server configuration
type SMTPConfig struct {
	Host         string        `json:"host" validate:"required"`
	Port         int           `json:"port" validate:"required,min=1,max=65535"`
	Username     string        `json:"username" validate:"required"`
	Password     string        `json:"password" validate:"required"`
	FromEmail    string        `json:"from_email" validate:"required,email"`
	FromName     string        `json:"from_name,omitempty"`
	UseTLS       bool          `json:"use_tls"`
	UseSSL       bool          `json:"use_ssl"`
	Timeout      time.Duration `json:"timeout"`
	MaxRetries   int           `json:"max_retries"`
	RetryDelay   time.Duration `json:"retry_delay"`
	PoolSize     int           `json:"pool_size"`
	RateLimitRPS int           `json:"rate_limit_rps"` // Rate limit: requests per second
}

// DefaultSMTPConfig returns default SMTP configuration
func DefaultSMTPConfig() *SMTPConfig {
	return &SMTPConfig{
		Port:         587,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		PoolSize:     10,
		RateLimitRPS: 5,
	}
}

// smtpSender implements EmailSender interface
type smtpSender struct {
	config      *SMTPConfig
	templates   map[string]*EmailTemplate
	auth        smtp.Auth
	rateLimiter chan struct{} // Rate limiting channel
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	Name         string `json:"name"`
	Subject      string `json:"subject"`
	HTMLTemplate *template.Template
	TextTemplate *template.Template
}

// NewEmailSender creates a new SMTP email sender
func NewEmailSender(config *SMTPConfig) (EmailSender, error) {
	if config == nil {
		return nil, fmt.Errorf("SMTP config is required")
	}

	// Validate configuration
	if err := validateSMTPConfig(config); err != nil {
		return nil, fmt.Errorf("invalid SMTP config: %w", err)
	}

	// Setup authentication
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	// Create rate limiter channel
	rateLimiter := make(chan struct{}, config.PoolSize)
	for i := 0; i < config.PoolSize; i++ {
		rateLimiter <- struct{}{}
	}

	sender := &smtpSender{
		config:      config,
		auth:        auth,
		templates:   make(map[string]*EmailTemplate),
		rateLimiter: rateLimiter,
	}

	return sender, nil
}

// ValidateConfig validates SMTP configuration
func (s *smtpSender) ValidateConfig() error {
	return validateSMTPConfig(s.config)
}

// validateSMTPConfig validates SMTP configuration
func validateSMTPConfig(config *SMTPConfig) error {
	if config.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("invalid SMTP port: %d", config.Port)
	}
	if config.Username == "" {
		return fmt.Errorf("SMTP username is required")
	}
	if config.Password == "" {
		return fmt.Errorf("SMTP password is required")
	}
	if config.FromEmail == "" {
		return fmt.Errorf("from email is required")
	}
	if !isValidEmail(config.FromEmail) {
		return fmt.Errorf("invalid from email format: %s", config.FromEmail)
	}
	if config.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	if config.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative")
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	return nil
}

// SendEmail sends a simple email
func (s *smtpSender) SendEmail(req *SendEmailRequest) error {
	if err := s.validateSendEmailRequest(req); err != nil {
		return fmt.Errorf("invalid email request: %w", err)
	}

	// Build email message
	message, err := s.buildEmailMessage(req.To, req.CC, req.BCC, req.Subject, req.HTMLBody, req.TextBody)
	if err != nil {
		return fmt.Errorf("failed to build email message: %w", err)
	}

	// Get all recipients
	allRecipients := append(req.To, req.CC...)
	allRecipients = append(allRecipients, req.BCC...)

	// Send with retry mechanism
	return s.sendWithRetry(allRecipients, message, req.Priority)
}

// SendTemplatedEmail sends an email using a template
func (s *smtpSender) SendTemplatedEmail(req *TemplatedEmailRequest) error {
	if err := s.validateTemplatedEmailRequest(req); err != nil {
		return fmt.Errorf("invalid templated email request: %w", err)
	}

	// Get template
	tmpl, exists := s.templates[req.TemplateName]
	if !exists {
		return fmt.Errorf("template not found: %s", req.TemplateName)
	}

	// Render subject
	subject, err := s.renderTemplate(tmpl.Subject, req.Variables)
	if err != nil {
		return fmt.Errorf("failed to render subject template: %w", err)
	}

	// Render HTML body
	var htmlBody string
	if tmpl.HTMLTemplate != nil {
		htmlBody, err = s.renderHTMLTemplate(tmpl.HTMLTemplate, req.Variables)
		if err != nil {
			return fmt.Errorf("failed to render HTML template: %w", err)
		}
	}

	// Render text body
	var textBody string
	if tmpl.TextTemplate != nil {
		textBody, err = s.renderHTMLTemplate(tmpl.TextTemplate, req.Variables)
		if err != nil {
			return fmt.Errorf("failed to render text template: %w", err)
		}
	}

	// Build email message
	message, err := s.buildEmailMessage(req.To, req.CC, req.BCC, subject, htmlBody, textBody)
	if err != nil {
		return fmt.Errorf("failed to build email message: %w", err)
	}

	// Get all recipients
	allRecipients := append(req.To, req.CC...)
	allRecipients = append(allRecipients, req.BCC...)

	// Send with retry mechanism
	return s.sendWithRetry(allRecipients, message, req.Priority)
}

// SendBulkEmail sends bulk emails
func (s *smtpSender) SendBulkEmail(req *BulkEmailRequest) error {
	if err := s.validateBulkEmailRequest(req); err != nil {
		return fmt.Errorf("invalid bulk email request: %w", err)
	}

	var errors []string

	for _, recipient := range req.Recipients {
		// Render personalized content if variables provided
		subject := req.Subject
		htmlBody := req.HTMLBody
		textBody := req.TextBody

		if len(recipient.Variables) > 0 {
			var err error
			subject, err = s.renderTemplate(subject, recipient.Variables)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to render subject for %s: %v", recipient.Email, err))
				continue
			}

			htmlBody, err = s.renderTemplate(htmlBody, recipient.Variables)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to render HTML body for %s: %v", recipient.Email, err))
				continue
			}

			textBody, err = s.renderTemplate(textBody, recipient.Variables)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to render text body for %s: %v", recipient.Email, err))
				continue
			}
		}

		// Build message
		message, err := s.buildEmailMessage([]string{recipient.Email}, nil, nil, subject, htmlBody, textBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to build message for %s: %v", recipient.Email, err))
			continue
		}

		// Send email
		if err := s.sendWithRetry([]string{recipient.Email}, message, req.Priority); err != nil {
			errors = append(errors, fmt.Sprintf("failed to send email to %s: %v", recipient.Email, err))
		}

		// Rate limiting
		time.Sleep(time.Second / time.Duration(s.config.RateLimitRPS))
	}

	if len(errors) > 0 {
		return fmt.Errorf("bulk email errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// sendWithRetry sends email with retry mechanism
func (s *smtpSender) sendWithRetry(recipients []string, message []byte, priority models.NotificationPriority) error {
	var lastErr error

	// Acquire rate limiter token
	<-s.rateLimiter
	defer func() {
		s.rateLimiter <- struct{}{}
	}()

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay with jitter
			delay := time.Duration(attempt) * s.config.RetryDelay

			// Add priority-based delay adjustment
			switch priority {
			case models.NotificationPriorityCritical:
				delay = delay / 2 // Faster retry for critical emails
			case models.NotificationPriorityLow:
				delay = delay * 2 // Slower retry for low priority emails
			}

			logger.WithFields(map[string]interface{}{
				"attempt":    attempt,
				"delay":      delay,
				"recipients": len(recipients),
				"priority":   priority,
			}).Info("Retrying email send")

			time.Sleep(delay)
		}

		err := s.sendSMTP(recipients, message)
		if err == nil {
			if attempt > 0 {
				logger.WithFields(map[string]interface{}{
					"attempt":    attempt,
					"recipients": len(recipients),
				}).Info("Email sent successfully after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !s.isRetryableError(err) {
			logger.WithFields(map[string]interface{}{
				"error":      err.Error(),
				"recipients": len(recipients),
			}).Error("Non-retryable email error")
			break
		}

		logger.WithFields(map[string]interface{}{
			"attempt":    attempt,
			"error":      err.Error(),
			"recipients": len(recipients),
		}).Warn("Email send attempt failed")
	}

	return fmt.Errorf("failed to send email after %d attempts: %w", s.config.MaxRetries+1, lastErr)
}

// sendSMTP sends email via SMTP
func (s *smtpSender) sendSMTP(recipients []string, message []byte) error {
	// Build server address
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Handle SSL/TLS connection
	if s.config.UseSSL {
		return s.sendWithSSL(addr, recipients, message)
	}

	return s.sendWithTLS(addr, recipients, message)
}

// sendWithTLS sends email with STARTTLS
func (s *smtpSender) sendWithTLS(addr string, recipients []string, message []byte) error {
	// Connect to server
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Start TLS if enabled
	if s.config.UseTLS {
		tlsConfig := &tls.Config{
			ServerName: s.config.Host,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate
	if err := client.Auth(s.auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set sender
	if err := client.Mail(s.config.FromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Send message
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data transmission: %w", err)
	}
	defer writer.Close()

	if _, err := writer.Write(message); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// sendWithSSL sends email with SSL/TLS connection
func (s *smtpSender) sendWithSSL(addr string, recipients []string, message []byte) error {
	// Use smtp.SendMail for SSL
	return smtp.SendMail(addr, s.auth, s.config.FromEmail, recipients, message)
}

// buildEmailMessage builds the email message
func (s *smtpSender) buildEmailMessage(to, cc, bcc []string, subject, htmlBody, textBody string) ([]byte, error) {
	var msg bytes.Buffer

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", s.formatFromAddress()))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))

	if len(cc) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(cc, ", ")))
	}

	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")

	// Content type based on available bodies
	if htmlBody != "" && textBody != "" {
		// Multipart alternative
		boundary := fmt.Sprintf("boundary_%d", time.Now().Unix())
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))

		// Text part
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		msg.WriteString(textBody)
		msg.WriteString("\r\n\r\n")

		// HTML part
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		msg.WriteString(htmlBody)
		msg.WriteString("\r\n\r\n")

		msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else if htmlBody != "" {
		// HTML only
		msg.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		msg.WriteString(htmlBody)
		msg.WriteString("\r\n")
	} else if textBody != "" {
		// Text only
		msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		msg.WriteString(textBody)
		msg.WriteString("\r\n")
	} else {
		return nil, fmt.Errorf("either HTML or text body must be provided")
	}

	return msg.Bytes(), nil
}

// formatFromAddress formats the from address with name
func (s *smtpSender) formatFromAddress() string {
	if s.config.FromName != "" {
		return fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromEmail)
	}
	return s.config.FromEmail
}

// renderTemplate renders a template string with variables
func (s *smtpSender) renderTemplate(templateStr string, variables map[string]interface{}) (string, error) {
	if variables == nil || len(variables) == 0 {
		return templateStr, nil
	}

	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// renderHTMLTemplate renders an HTML template with variables
func (s *smtpSender) renderHTMLTemplate(tmpl *template.Template, variables map[string]interface{}) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %w", err)
	}
	return buf.String(), nil
}

// isRetryableError checks if an error is retryable
func (s *smtpSender) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network errors are usually retryable
	retryableErrors := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no such host",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"server misbehaving",
		"too many connections",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	// SMTP temporary failures (4xx codes) are retryable
	if strings.Contains(errStr, "450 ") || strings.Contains(errStr, "451 ") ||
		strings.Contains(errStr, "452 ") || strings.Contains(errStr, "421 ") {
		return true
	}

	// SMTP permanent failures (5xx codes) are usually not retryable
	if strings.Contains(errStr, "550 ") || strings.Contains(errStr, "551 ") ||
		strings.Contains(errStr, "552 ") || strings.Contains(errStr, "553 ") ||
		strings.Contains(errStr, "554 ") {
		return false
	}

	return false
}

// Validation methods

func (s *smtpSender) validateSendEmailRequest(req *SendEmailRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if len(req.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	for _, email := range req.To {
		if !isValidEmail(email) {
			return fmt.Errorf("invalid email address: %s", email)
		}
	}
	for _, email := range req.CC {
		if !isValidEmail(email) {
			return fmt.Errorf("invalid CC email address: %s", email)
		}
	}
	for _, email := range req.BCC {
		if !isValidEmail(email) {
			return fmt.Errorf("invalid BCC email address: %s", email)
		}
	}
	if strings.TrimSpace(req.Subject) == "" {
		return fmt.Errorf("subject is required")
	}
	if req.HTMLBody == "" && req.TextBody == "" {
		return fmt.Errorf("either HTML or text body is required")
	}
	return nil
}

func (s *smtpSender) validateTemplatedEmailRequest(req *TemplatedEmailRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if len(req.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	for _, email := range req.To {
		if !isValidEmail(email) {
			return fmt.Errorf("invalid email address: %s", email)
		}
	}
	if strings.TrimSpace(req.TemplateName) == "" {
		return fmt.Errorf("template name is required")
	}
	return nil
}

func (s *smtpSender) validateBulkEmailRequest(req *BulkEmailRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if len(req.Recipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if len(req.Recipients) > 1000 {
		return fmt.Errorf("too many recipients (max 1000)")
	}
	for i, recipient := range req.Recipients {
		if !isValidEmail(recipient.Email) {
			return fmt.Errorf("invalid email address at index %d: %s", i, recipient.Email)
		}
	}
	if strings.TrimSpace(req.Subject) == "" {
		return fmt.Errorf("subject is required")
	}
	if req.HTMLBody == "" && req.TextBody == "" {
		return fmt.Errorf("either HTML or text body is required")
	}
	return nil
}

// Template management methods

// LoadTemplate loads an email template
func (s *smtpSender) LoadTemplate(tmpl *models.EmailTemplate) error {
	if tmpl == nil {
		return fmt.Errorf("template is required")
	}

	emailTemplate := &EmailTemplate{
		Name:    tmpl.Name,
		Subject: tmpl.Subject,
	}

	// Parse HTML template
	if tmpl.HTMLTemplate != "" {
		htmlTmpl, err := template.New(tmpl.Name + "_html").Parse(tmpl.HTMLTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse HTML template: %w", err)
		}
		emailTemplate.HTMLTemplate = htmlTmpl
	}

	// Parse text template
	if tmpl.TextTemplate != "" {
		textTmpl, err := template.New(tmpl.Name + "_text").Parse(tmpl.TextTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse text template: %w", err)
		}
		emailTemplate.TextTemplate = textTmpl
	}

	s.templates[tmpl.Name] = emailTemplate
	return nil
}

// RemoveTemplate removes an email template
func (s *smtpSender) RemoveTemplate(name string) {
	delete(s.templates, name)
}

// GetTemplateNames returns list of loaded template names
func (s *smtpSender) GetTemplateNames() []string {
	names := make([]string, 0, len(s.templates))
	for name := range s.templates {
		names = append(names, name)
	}
	return names
}

// Utility functions

// isValidEmail validates email format
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}

	// Basic email validation
	atCount := strings.Count(email, "@")
	if atCount != 1 {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}

	// Check for dots in domain
	if !strings.Contains(parts[1], ".") {
		return false
	}

	return true
}

// GetSMTPConfigFromEnv creates SMTP config from environment variables
func GetSMTPConfigFromEnv() *SMTPConfig {
	config := DefaultSMTPConfig()

	if host := getEnv("SMTP_HOST", ""); host != "" {
		config.Host = host
	}

	if portStr := getEnv("SMTP_PORT", ""); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	if username := getEnv("SMTP_USERNAME", ""); username != "" {
		config.Username = username
	}

	if password := getEnv("SMTP_PASSWORD", ""); password != "" {
		config.Password = password
	}

	if fromEmail := getEnv("SMTP_FROM_EMAIL", ""); fromEmail != "" {
		config.FromEmail = fromEmail
	}

	if fromName := getEnv("SMTP_FROM_NAME", ""); fromName != "" {
		config.FromName = fromName
	}

	if useTLS := getEnv("SMTP_USE_TLS", "true"); useTLS == "false" {
		config.UseTLS = false
	}

	if useSSL := getEnv("SMTP_USE_SSL", "false"); useSSL == "true" {
		config.UseSSL = true
	}

	if timeoutStr := getEnv("SMTP_TIMEOUT_SECONDS", ""); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.Timeout = time.Duration(timeout) * time.Second
		}
	}

	if retriesStr := getEnv("SMTP_MAX_RETRIES", ""); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil {
			config.MaxRetries = retries
		}
	}

	if delayStr := getEnv("SMTP_RETRY_DELAY_SECONDS", ""); delayStr != "" {
		if delay, err := strconv.Atoi(delayStr); err == nil {
			config.RetryDelay = time.Duration(delay) * time.Second
		}
	}

	if poolStr := getEnv("SMTP_POOL_SIZE", ""); poolStr != "" {
		if pool, err := strconv.Atoi(poolStr); err == nil {
			config.PoolSize = pool
		}
	}

	if rpsStr := getEnv("SMTP_RATE_LIMIT_RPS", ""); rpsStr != "" {
		if rps, err := strconv.Atoi(rpsStr); err == nil {
			config.RateLimitRPS = rps
		}
	}

	return config
}

// Helper function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
