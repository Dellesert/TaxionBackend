// File: services/notification/email/config_example.go
package email

import (
	"fmt"
	"strings"
	"time"
)

// Примеры конфигурации для популярных SMTP провайдеров

// GmailSMTPConfig возвращает конфигурацию для Gmail SMTP
func GmailSMTPConfig(username, password, fromEmail, fromName string) *SMTPConfig {
	return &SMTPConfig{
		Host:         "smtp.gmail.com",
		Port:         587,
		Username:     username, // ваш Gmail адрес
		Password:     password, // пароль приложения (не основной пароль)
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		PoolSize:     10,
		RateLimitRPS: 14, // Gmail лимит: 14 писем в секунду
	}
}

// OutlookSMTPConfig возвращает конфигурацию для Outlook/Hotmail SMTP
func OutlookSMTPConfig(username, password, fromEmail, fromName string) *SMTPConfig {
	return &SMTPConfig{
		Host:         "smtp-mail.outlook.com",
		Port:         587,
		Username:     username,
		Password:     password,
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		PoolSize:     10,
		RateLimitRPS: 5, // Консервативный лимит
	}
}

// YandexSMTPConfig возвращает конфигурацию для Yandex SMTP
func YandexSMTPConfig(username, password, fromEmail, fromName string) *SMTPConfig {
	return &SMTPConfig{
		Host:         "smtp.yandex.ru",
		Port:         587,
		Username:     username,
		Password:     password, // пароль приложения
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		PoolSize:     10,
		RateLimitRPS: 10,
	}
}

// MailRuSMTPConfig возвращает конфигурацию для Mail.ru SMTP
func MailRuSMTPConfig(username, password, fromEmail, fromName string) *SMTPConfig {
	return &SMTPConfig{
		Host:         "smtp.mail.ru",
		Port:         587,
		Username:     username,
		Password:     password,
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		PoolSize:     10,
		RateLimitRPS: 5,
	}
}

// SendGridSMTPConfig возвращает конфигурацию для SendGrid SMTP
func SendGridSMTPConfig(apiKey, fromEmail, fromName string) *SMTPConfig {
	return &SMTPConfig{
		Host:         "smtp.sendgrid.net",
		Port:         587,
		Username:     "apikey", // SendGrid использует "apikey" как username
		Password:     apiKey,   // ваш SendGrid API ключ
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
		PoolSize:     20,
		RateLimitRPS: 100, // SendGrid поддерживает высокие лимиты
	}
}

// MailgunSMTPConfig возвращает конфигурацию для Mailgun SMTP
func MailgunSMTPConfig(domain, username, password, fromEmail, fromName string) *SMTPConfig {
	return &SMTPConfig{
		Host:         "smtp.mailgun.org",
		Port:         587,
		Username:     username, // обычно postmaster@yourdomain.com
		Password:     password, // ваш Mailgun SMTP пароль
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
		PoolSize:     20,
		RateLimitRPS: 100,
	}
}

// AmazonSESSMTPConfig возвращает конфигурацию для Amazon SES SMTP
func AmazonSESSMTPConfig(region, username, password, fromEmail, fromName string) *SMTPConfig {
	// Определяем SMTP endpoint на основе региона
	host := fmt.Sprintf("email-smtp.%s.amazonaws.com", region)

	return &SMTPConfig{
		Host:         host,
		Port:         587,
		Username:     username, // SMTP username из AWS SES
		Password:     password, // SMTP password из AWS SES
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       true,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
		PoolSize:     50,
		RateLimitRPS: 200, // SES поддерживает высокие лимиты
	}
}

// CustomSMTPConfig создает конфигурацию для кастомного SMTP сервера
func CustomSMTPConfig(host string, port int, username, password, fromEmail, fromName string, useTLS bool) *SMTPConfig {
	return &SMTPConfig{
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		FromEmail:    fromEmail,
		FromName:     fromName,
		UseTLS:       useTLS,
		UseSSL:       false,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		PoolSize:     10,
		RateLimitRPS: 5,
	}
}

// Рекомендации по использованию различных провайдеров

/*
GMAIL SMTP:
- Необходимо включить "Двухфакторную аутентификацию"
- Создать "Пароль приложения" в настройках Google
- Лимит: 500 писем в день для бесплатных аккаунтов
- Лимит: до 14 писем в секунду

OUTLOOK/HOTMAIL SMTP:
- Использовать основной пароль или пароль приложения
- Лимит: 300 писем в день для бесплатных аккаунтов
- Лимит скорости: 5-10 писем в минуту

YANDEX SMTP:
- Необходимо включить "Доступ по протоколу IMAP"
- Создать пароль приложения в настройках безопасности
- Лимит: 500 писем в день

MAIL.RU SMTP:
- Включить поддержку внешних клиентов в настройках
- Лимит: 300 писем в день

SENDGRID SMTP:
- Коммерческий сервис с высокими лимитами
- Требует верификации домена
- Отличная доставляемость

MAILGUN SMTP:
- Коммерческий сервис
- Sandbox режим для тестирования
- API и SMTP интерфейсы

AMAZON SES SMTP:
- Очень высокие лимиты
- Необходима верификация email адресов/доменов
- Sandbox режим по умолчанию (требует запроса на снятие)

НАСТРОЙКИ ПЕРЕМЕННЫХ ОКРУЖЕНИЯ:
Добавьте в .env файл:

# SMTP Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM_EMAIL=your-email@gmail.com
SMTP_FROM_NAME=Tachyon Messenger
SMTP_USE_TLS=true
SMTP_USE_SSL=false
SMTP_TIMEOUT_SECONDS=30
SMTP_MAX_RETRIES=3
SMTP_RETRY_DELAY_SECONDS=5
SMTP_POOL_SIZE=10
SMTP_RATE_LIMIT_RPS=5

ПРИМЕР ИСПОЛЬЗОВАНИЯ:

package main

import (
    "log"
    "tachyon-messenger/services/notification/email"
)

func main() {
    // Вариант 1: Из переменных окружения
    config := email.GetSMTPConfigFromEnv()

    // Вариант 2: Gmail конфигурация
    config := email.GmailSMTPConfig(
        "your-email@gmail.com",
        "your-app-password",
        "noreply@yourcompany.com",
        "Your Company",
    )

    // Создание sender'а
    sender, err := email.NewEmailSender(config)
    if err != nil {
        log.Fatal("Failed to create email sender:", err)
    }

    // Загрузка шаблонов
    loader := email.NewTemplateLoader(sender)
    if err := loader.LoadDefaultTemplates(); err != nil {
        log.Fatal("Failed to load templates:", err)
    }

    // Отправка простого email
    err = sender.SendEmail(&email.SendEmailRequest{
        To:       []string{"user@example.com"},
        Subject:  "Test Email",
        HTMLBody: "<h1>Hello World!</h1>",
        TextBody: "Hello World!",
    })

    if err != nil {
        log.Fatal("Failed to send email:", err)
    }

    // Отправка с использованием шаблона
    err = sender.SendTemplatedEmail(&email.TemplatedEmailRequest{
        To:           []string{"user@example.com"},
        TemplateName: "welcome",
        Variables: map[string]interface{}{
            "UserName": "John Doe",
            "AppURL":   "https://your-app.com",
        },
    })

    if err != nil {
        log.Fatal("Failed to send templated email:", err)
    }
}

БЕЗОПАСНОСТЬ:
- Никогда не храните пароли в коде
- Используйте переменные окружения
- Для production используйте пароли приложений или API ключи
- Регулярно обновляйте пароли
- Мониторьте логи на предмет ошибок аутентификации

МОНИТОРИНГ:
- Логируйте все попытки отправки
- Отслеживайте rate limits
- Мониторьте bounce и complaint rates
- Настройте алерты на ошибки SMTP

ОТЛАДКА:
- Проверьте настройки брандмауэра (порты 587, 465, 25)
- Убедитесь в правильности учетных данных
- Проверьте DNS настройки
- Используйте telnet для тестирования подключения: telnet smtp.gmail.com 587
*/

// TestSMTPConfig тестирует SMTP конфигурацию
func TestSMTPConfig(config *SMTPConfig) error {
	sender, err := NewEmailSender(config)
	if err != nil {
		return fmt.Errorf("failed to create sender: %w", err)
	}

	return sender.ValidateConfig()
}

// GetRecommendedConfig возвращает рекомендованную конфигурацию на основе email домена
func GetRecommendedConfig(emailAddress, password, fromName string) *SMTPConfig {
	domain := strings.ToLower(strings.Split(emailAddress, "@")[1])

	switch domain {
	case "gmail.com":
		return GmailSMTPConfig(emailAddress, password, emailAddress, fromName)
	case "outlook.com", "hotmail.com", "live.com":
		return OutlookSMTPConfig(emailAddress, password, emailAddress, fromName)
	case "yandex.ru", "yandex.com", "ya.ru":
		return YandexSMTPConfig(emailAddress, password, emailAddress, fromName)
	case "mail.ru", "inbox.ru", "list.ru", "bk.ru":
		return MailRuSMTPConfig(emailAddress, password, emailAddress, fromName)
	default:
		// Возвращаем базовую конфигурацию для неизвестных доменов
		return &SMTPConfig{
			Host:         "smtp." + domain,
			Port:         587,
			Username:     emailAddress,
			Password:     password,
			FromEmail:    emailAddress,
			FromName:     fromName,
			UseTLS:       true,
			UseSSL:       false,
			Timeout:      30 * time.Second,
			MaxRetries:   3,
			RetryDelay:   5 * time.Second,
			PoolSize:     10,
			RateLimitRPS: 5,
		}
	}
}
