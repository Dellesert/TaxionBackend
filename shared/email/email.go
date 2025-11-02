package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strings"
)

// EmailConfig holds email configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

// EmailService handles email operations
type EmailService struct {
	config *EmailConfig
}

// NewEmailService creates a new email service
func NewEmailService(config *EmailConfig) *EmailService {
	return &EmailService{
		config: config,
	}
}

// LoadConfigFromEnv loads email configuration from environment variables
func LoadConfigFromEnv() *EmailConfig {
	return &EmailConfig{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		FromEmail:    os.Getenv("SMTP_FROM_EMAIL"),
		FromName:     getEnvOrDefault("SMTP_FROM_NAME", "Tachyon Messenger"),
	}
}

// SendEmail sends an email
func (s *EmailService) SendEmail(to, subject, htmlBody string) error {
	// Check if we're in development mode (log emails instead of sending)
	if os.Getenv("EMAIL_MODE") == "log" || (os.Getenv("ENVIRONMENT") == "development" && s.config.SMTPHost == "") {
		fmt.Printf("\n========== EMAIL (NOT SENT - LOG MODE) ==========\n")
		fmt.Printf("To: %s\n", to)
		fmt.Printf("Subject: %s\n", subject)
		fmt.Printf("From: %s <%s>\n", s.config.FromName, s.config.FromEmail)
		fmt.Printf("Body (truncated): %s...\n", htmlBody[:min(200, len(htmlBody))])
		fmt.Printf("=================================================\n\n")
		return nil
	}

	// Validate configuration
	if err := s.validateConfig(); err != nil {
		return fmt.Errorf("invalid email configuration: %w", err)
	}

	// Set up authentication (nil for MailHog/testing servers)
	var auth smtp.Auth
	// MailHog and some test servers don't need authentication
	if s.config.SMTPUser != "test" && s.config.SMTPPassword != "test" {
		auth = smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPassword, s.config.SMTPHost)
	}

	// Compose email
	from := s.config.FromEmail
	if s.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromEmail)
	}

	// Build email headers and body
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	// Compose message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	// Send email
	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)
	err := smtp.SendMail(addr, auth, s.config.FromEmail, []string{to}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Send2FACode sends a 2FA code via email
func (s *EmailService) Send2FACode(to, code, userName string) error {
	subject := "Ваш код двухфакторной аутентификации"

	htmlBody, err := s.render2FATemplate(code, userName)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.SendEmail(to, subject, htmlBody)
}

// render2FATemplate renders the 2FA email template
func (s *EmailService) render2FATemplate(code, userName string) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Код двухфакторной аутентификации</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f4f4f4;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #2c3e50;
            margin: 0;
            font-size: 24px;
        }
        .code-box {
            background-color: #f8f9fa;
            border: 2px solid #e9ecef;
            border-radius: 8px;
            padding: 30px;
            text-align: center;
            margin: 30px 0;
        }
        .code {
            font-size: 36px;
            font-weight: bold;
            color: #3498db;
            letter-spacing: 8px;
            font-family: 'Courier New', monospace;
        }
        .message {
            color: #555;
            margin: 20px 0;
            text-align: center;
        }
        .warning {
            background-color: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 4px;
            padding: 15px;
            margin: 20px 0;
            color: #856404;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e9ecef;
            color: #6c757d;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔐 Двухфакторная аутентификация</h1>
        </div>

        <p class="message">
            Здравствуйте{{if .UserName}}, <strong>{{.UserName}}</strong>{{end}}!
        </p>

        <p class="message">
            Ваш код для входа в систему Tachyon Messenger:
        </p>

        <div class="code-box">
            <div class="code">{{.Code}}</div>
        </div>

        <p class="message">
            Введите этот код на странице входа для завершения авторизации.
        </p>

        <div class="warning">
            <strong>⚠️ Важно:</strong>
            <ul style="margin: 10px 0; padding-left: 20px;">
                <li>Код действителен в течение <strong>5 минут</strong></li>
                <li>Никому не сообщайте этот код</li>
                <li>Если вы не запрашивали этот код, проигнорируйте это письмо</li>
            </ul>
        </div>

        <div class="footer">
            <p>С уважением,<br>Команда Tachyon Messenger</p>
            <p style="font-size: 12px; color: #adb5bd;">
                Это автоматическое сообщение, пожалуйста, не отвечайте на него.
            </p>
        </div>
    </div>
</body>
</html>
`

	// Parse and execute template
	t, err := template.New("2fa").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := struct {
		Code     string
		UserName string
	}{
		Code:     code,
		UserName: userName,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// validateConfig validates email configuration
func (s *EmailService) validateConfig() error {
	if s.config.SMTPHost == "" {
		return fmt.Errorf("SMTP_HOST is required")
	}
	if s.config.SMTPPort == "" {
		return fmt.Errorf("SMTP_PORT is required")
	}
	if s.config.SMTPUser == "" {
		return fmt.Errorf("SMTP_USER is required")
	}
	if s.config.SMTPPassword == "" {
		return fmt.Errorf("SMTP_PASSWORD is required")
	}
	if s.config.FromEmail == "" {
		return fmt.Errorf("SMTP_FROM_EMAIL is required")
	}
	return nil
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

// SendPasswordResetEmail sends a password reset email
func (s *EmailService) SendPasswordResetEmail(to, resetLink, userName string) error {
	subject := "Сброс пароля Tachyon Messenger"

	htmlBody, err := s.renderPasswordResetTemplate(resetLink, userName)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.SendEmail(to, subject, htmlBody)
}

// renderPasswordResetTemplate renders the password reset email template
func (s *EmailService) renderPasswordResetTemplate(resetLink, userName string) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Сброс пароля</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f4f4f4;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #2c3e50;
            margin: 0;
            font-size: 24px;
        }
        .message {
            color: #555;
            margin: 20px 0;
            line-height: 1.8;
        }
        .button-container {
            text-align: center;
            margin: 40px 0;
        }
        .reset-button {
            display: inline-block;
            background-color: #3498db;
            color: #ffffff !important;
            text-decoration: none;
            padding: 15px 40px;
            border-radius: 6px;
            font-weight: bold;
            font-size: 16px;
        }
        .reset-button:hover {
            background-color: #2980b9;
        }
        .warning {
            background-color: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 4px;
            padding: 15px;
            margin: 20px 0;
            color: #856404;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e9ecef;
            color: #6c757d;
            font-size: 14px;
        }
        .link-fallback {
            margin-top: 20px;
            padding: 15px;
            background-color: #f8f9fa;
            border-radius: 4px;
            word-break: break-all;
            font-size: 12px;
            color: #6c757d;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔑 Сброс пароля</h1>
        </div>

        <p class="message">
            Здравствуйте{{if .UserName}}, <strong>{{.UserName}}</strong>{{end}}!
        </p>

        <p class="message">
            Мы получили запрос на сброс пароля для вашей учетной записи Tachyon Messenger.
        </p>

        <div class="button-container">
            <a href="{{.ResetLink}}" class="reset-button" target="_blank" rel="noopener">Сбросить пароль</a>
        </div>

        <p class="message" style="text-align: center; font-size: 13px; color: #6c757d;">
            Нажмите на кнопку выше, чтобы открыть приложение
        </p>

        <div class="warning">
            <strong>⚠️ Важно:</strong>
            <ul style="margin: 10px 0; padding-left: 20px;">
                <li>Ссылка действительна в течение <strong>24 часов</strong></li>
                <li>Ссылка может быть использована только один раз</li>
                <li>Если вы не запрашивали сброс пароля, проигнорируйте это письмо</li>
                <li>Никому не передавайте эту ссылку</li>
            </ul>
        </div>

        <div class="link-fallback">
            <p style="margin: 0 0 10px 0;"><strong>📱 Для мобильного приложения:</strong></p>
            <p style="margin: 5px 0; font-size: 13px;">Скопируйте ссылку ниже и откройте её в приложении Tachyon Messenger:</p>
            <div style="background: white; padding: 10px; border: 1px solid #dee2e6; border-radius: 4px; margin: 10px 0;">
                <code style="color: #3498db; word-break: break-all;">{{.ResetLink}}</code>
            </div>
            <p style="margin: 10px 0 0 0; font-size: 12px; color: #6c757d;">
                Или нажмите на кнопку "Сбросить пароль" выше
            </p>
        </div>

        <div class="footer">
            <p>С уважением,<br>Команда Tachyon Messenger</p>
            <p style="font-size: 12px; color: #adb5bd;">
                Это автоматическое сообщение, пожалуйста, не отвечайте на него.
            </p>
        </div>
    </div>
</body>
</html>
`

	// Parse and execute template
	t, err := template.New("password_reset").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := struct {
		ResetLink string
		UserName  string
	}{
		ResetLink: resetLink,
		UserName:  userName,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
