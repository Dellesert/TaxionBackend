// File: services/notification/email/templates.go
package email

import (
	"fmt"
	"html/template"
	"strings"

	"tachyon-messenger/services/notification/models"
)

// DefaultEmailTemplates contains built-in email templates
var DefaultEmailTemplates = map[string]*models.EmailTemplate{
	"welcome": {
		Name:    "welcome",
		Type:    models.NotificationTypeSystem,
		Subject: "Добро пожаловать в Tachyon Messenger, {{.UserName}}!",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Добро пожаловать в Tachyon Messenger</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #007bff; }
        .logo { font-size: 28px; font-weight: bold; color: #007bff; }
        .content { padding: 30px 0; }
        .welcome-title { font-size: 24px; color: #333; margin-bottom: 20px; }
        .welcome-text { font-size: 16px; line-height: 1.8; margin-bottom: 20px; }
        .cta-button { display: inline-block; background-color: #007bff; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .cta-button:hover { background-color: #0056b3; }
        .features { margin: 30px 0; }
        .feature { margin: 15px 0; padding: 15px; background-color: #f8f9fa; border-left: 4px solid #007bff; }
        .feature-title { font-weight: bold; color: #007bff; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
        </div>
        
        <div class="content">
            <h1 class="welcome-title">Добро пожаловать, {{.UserName}}! 🎉</h1>
            
            <p class="welcome-text">
                Мы рады приветствовать вас в корпоративном сообществе <strong>Tachyon Messenger</strong>! 
                Ваш аккаунт успешно создан и готов к использованию.
            </p>
            
            <p class="welcome-text">
                Теперь вы можете общаться с коллегами, управлять задачами и быть в курсе всех важных событий компании.
            </p>
            
            <div style="text-align: center;">
                <a href="{{.AppURL}}" class="cta-button">Начать работу</a>
            </div>
            
            <div class="features">
                <h3>Возможности Tachyon Messenger:</h3>
                
                <div class="feature">
                    <div class="feature-title">💬 Мессенджер</div>
                    Общайтесь с коллегами в личных и групповых чатах
                </div>
                
                <div class="feature">
                    <div class="feature-title">📋 Управление задачами</div>
                    Создавайте, назначайте и отслеживайте выполнение задач
                </div>
                
                <div class="feature">
                    <div class="feature-title">📅 Календарь</div>
                    Планируйте встречи и следите за важными событиями
                </div>
                
                <div class="feature">
                    <div class="feature-title">📊 Опросы</div>
                    Участвуйте в корпоративных опросах и голосованиях
                </div>
            </div>
            
            <p class="welcome-text">
                Если у вас возникнут вопросы, не стесняйтесь обращаться к нашей службе поддержки.
            </p>
        </div>
        
        <div class="footer">
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
Добро пожаловать в Tachyon Messenger, {{.UserName}}!

Мы рады приветствовать вас в корпоративном сообществе Tachyon Messenger!
Ваш аккаунт успешно создан и готов к использованию.

Теперь вы можете общаться с коллегами, управлять задачами и быть в курсе всех важных событий компании.

Возможности Tachyon Messenger:
• Мессенджер - общайтесь с коллегами в личных и групповых чатах
• Управление задачами - создавайте, назначайте и отслеживайте выполнение задач
• Календарь - планируйте встречи и следите за важными событиями
• Опросы - участвуйте в корпоративных опросах и голосованиях

Начать работу: {{.AppURL}}

Если у вас возникнут вопросы, не стесняйтесь обращаться к нашей службе поддержки.

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},

	"task_assigned": {
		Name:    "task_assigned",
		Type:    models.NotificationTypeTask,
		Subject: "Вам назначена новая задача: {{.TaskTitle}}",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Новая задача</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #28a745; }
        .logo { font-size: 28px; font-weight: bold; color: #28a745; }
        .task-icon { font-size: 48px; margin: 20px 0; }
        .task-title { font-size: 24px; color: #333; margin: 20px 0; }
        .task-info { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin: 20px 0; }
        .info-row { display: flex; justify-content: space-between; margin: 10px 0; padding: 5px 0; border-bottom: 1px solid #eee; }
        .info-label { font-weight: bold; color: #666; }
        .priority-high { color: #dc3545; font-weight: bold; }
        .priority-medium { color: #ffc107; font-weight: bold; }
        .priority-low { color: #28a745; font-weight: bold; }
        .priority-critical { color: #6f42c1; font-weight: bold; }
        .cta-button { display: inline-block; background-color: #28a745; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
            <div class="task-icon">📋</div>
        </div>
        
        <div class="content">
            <h1 class="task-title">Вам назначена новая задача</h1>
            
            <div class="task-info">
                <div class="info-row">
                    <span class="info-label">Название:</span>
                    <span><strong>{{.TaskTitle}}</strong></span>
                </div>
                
                {{if .TaskDescription}}
                <div class="info-row">
                    <span class="info-label">Описание:</span>
                    <span>{{.TaskDescription}}</span>
                </div>
                {{end}}
                
                <div class="info-row">
                    <span class="info-label">Приоритет:</span>
                    <span class="priority-{{.TaskPriority}}">
                        {{if eq .TaskPriority "high"}}Высокий{{else if eq .TaskPriority "medium"}}Средний{{else if eq .TaskPriority "low"}}Низкий{{else if eq .TaskPriority "critical"}}Критический{{end}}
                    </span>
                </div>
                
                <div class="info-row">
                    <span class="info-label">Назначил:</span>
                    <span>{{.AssignerName}}</span>
                </div>
                
                {{if .DueDate}}
                <div class="info-row">
                    <span class="info-label">Срок выполнения:</span>
                    <span><strong>{{.DueDate}}</strong></span>
                </div>
                {{end}}
                
                <div class="info-row">
                    <span class="info-label">Дата создания:</span>
                    <span>{{.CreatedAt}}</span>
                </div>
            </div>
            
            <div style="text-align: center;">
                <a href="{{.TaskURL}}" class="cta-button">Просмотреть задачу</a>
            </div>
        </div>
        
        <div class="footer">
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
Вам назначена новая задача

Название: {{.TaskTitle}}
{{if .TaskDescription}}Описание: {{.TaskDescription}}{{end}}
Приоритет: {{if eq .TaskPriority "high"}}Высокий{{else if eq .TaskPriority "medium"}}Средний{{else if eq .TaskPriority "low"}}Низкий{{else if eq .TaskPriority "critical"}}Критический{{end}}
Назначил: {{.AssignerName}}
{{if .DueDate}}Срок выполнения: {{.DueDate}}{{end}}
Дата создания: {{.CreatedAt}}

Просмотреть задачу: {{.TaskURL}}

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},

	"message_notification": {
		Name:    "message_notification",
		Type:    models.NotificationTypeMessage,
		Subject: "Новое сообщение от {{.SenderName}}",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Новое сообщение</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #17a2b8; }
        .logo { font-size: 28px; font-weight: bold; color: #17a2b8; }
        .message-icon { font-size: 48px; margin: 20px 0; }
        .message-content { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #17a2b8; }
        .sender-info { margin-bottom: 15px; }
        .sender-name { font-weight: bold; color: #17a2b8; }
        .message-text { font-size: 16px; line-height: 1.6; }
        .chat-info { color: #666; font-size: 14px; margin-top: 15px; }
        .cta-button { display: inline-block; background-color: #17a2b8; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
            <div class="message-icon">💬</div>
        </div>
        
        <div class="content">
            <h1>Новое сообщение</h1>
            
            <div class="message-content">
                <div class="sender-info">
                    <span class="sender-name">{{.SenderName}}</span>
                    <span class="chat-info">в чате "{{.ChatName}}"</span>
                </div>
                
                <div class="message-text">
                    {{.MessageContent}}
                </div>
                
                <div class="chat-info">
                    {{.CreatedAt}}
                </div>
            </div>
            
            <div style="text-align: center;">
                <a href="{{.ChatURL}}" class="cta-button">Открыть чат</a>
            </div>
        </div>
        
        <div class="footer">
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
Новое сообщение от {{.SenderName}}

Чат: {{.ChatName}}
Сообщение: {{.MessageContent}}
Время: {{.CreatedAt}}

Открыть чат: {{.ChatURL}}

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},

	"calendar_reminder": {
		Name:    "calendar_reminder",
		Type:    models.NotificationTypeCalendar,
		Subject: "Напоминание о событии: {{.EventTitle}}",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Напоминание о событии</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #fd7e14; }
        .logo { font-size: 28px; font-weight: bold; color: #fd7e14; }
        .event-icon { font-size: 48px; margin: 20px 0; }
        .event-info { background-color: #fff3cd; padding: 20px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #fd7e14; }
        .info-row { display: flex; justify-content: space-between; margin: 10px 0; padding: 5px 0; border-bottom: 1px solid #f0f0f0; }
        .info-label { font-weight: bold; color: #666; }
        .cta-button { display: inline-block; background-color: #fd7e14; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
            <div class="event-icon">📅</div>
        </div>
        
        <div class="content">
            <h1>Напоминание о событии</h1>
            
            <div class="event-info">
                <div class="info-row">
                    <span class="info-label">Событие:</span>
                    <span><strong>{{.EventTitle}}</strong></span>
                </div>
                
                {{if .EventDescription}}
                <div class="info-row">
                    <span class="info-label">Описание:</span>
                    <span>{{.EventDescription}}</span>
                </div>
                {{end}}
                
                <div class="info-row">
                    <span class="info-label">Начало:</span>
                    <span><strong>{{.StartTime}}</strong></span>
                </div>
                
                {{if .EndTime}}
                <div class="info-row">
                    <span class="info-label">Окончание:</span>
                    <span>{{.EndTime}}</span>
                </div>
                {{end}}
                
                {{if .Location}}
                <div class="info-row">
                    <span class="info-label">Место:</span>
                    <span>{{.Location}}</span>
                </div>
                {{end}}
                
                {{if .Participants}}
                <div class="info-row">
                    <span class="info-label">Участники:</span>
                    <span>{{.Participants}}</span>
                </div>
                {{end}}
            </div>
            
            <div style="text-align: center;">
                <a href="{{.EventURL}}" class="cta-button">Просмотреть событие</a>
            </div>
        </div>
        
        <div class="footer">
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
Напоминание о событии: {{.EventTitle}}

{{if .EventDescription}}Описание: {{.EventDescription}}{{end}}
Начало: {{.StartTime}}
{{if .EndTime}}Окончание: {{.EndTime}}{{end}}
{{if .Location}}Место: {{.Location}}{{end}}
{{if .Participants}}Участники: {{.Participants}}{{end}}

Просмотреть событие: {{.EventURL}}

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},

	"system_announcement": {
		Name:    "system_announcement",
		Type:    models.NotificationTypeAnnounce,
		Subject: "{{.AnnouncementTitle}}",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Объявление</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #6f42c1; }
        .logo { font-size: 28px; font-weight: bold; color: #6f42c1; }
        .announcement-icon { font-size: 48px; margin: 20px 0; }
        .announcement-content { background-color: #f8f9ff; padding: 25px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #6f42c1; }
        .announcement-title { font-size: 24px; color: #6f42c1; margin-bottom: 15px; font-weight: bold; }
        .announcement-text { font-size: 16px; line-height: 1.8; }
        .important-notice { background-color: #fff3cd; padding: 15px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #ffc107; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
            <div class="announcement-icon">📢</div>
        </div>
        
        <div class="content">
            <div class="announcement-content">
                <h1 class="announcement-title">{{.AnnouncementTitle}}</h1>
                
                <div class="announcement-text">
                    {{.AnnouncementContent}}
                </div>
                
                {{if .IsImportant}}
                <div class="important-notice">
                    <strong>⚠️ Важное объявление!</strong> Пожалуйста, внимательно ознакомьтесь с данной информацией.
                </div>
                {{end}}
                
                {{if .ActionRequired}}
                <div class="important-notice">
                    <strong>📋 Требуется действие:</strong> {{.ActionRequired}}
                </div>
                {{end}}
            </div>
            
            {{if .ReadMoreURL}}
            <div style="text-align: center;">
                <a href="{{.ReadMoreURL}}" class="cta-button" style="display: inline-block; background-color: #6f42c1; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0;">Подробнее</a>
            </div>
            {{end}}
        </div>
        
        <div class="footer">
            <p>Дата публикации: {{.PublishedAt}}</p>
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
{{.AnnouncementTitle}}

{{.AnnouncementContent}}

{{if .IsImportant}}⚠️ ВАЖНОЕ ОБЪЯВЛЕНИЕ! Пожалуйста, внимательно ознакомьтесь с данной информацией.{{end}}

{{if .ActionRequired}}📋 ТРЕБУЕТСЯ ДЕЙСТВИЕ: {{.ActionRequired}}{{end}}

{{if .ReadMoreURL}}Подробнее: {{.ReadMoreURL}}{{end}}

Дата публикации: {{.PublishedAt}}

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},

	"poll_notification": {
		Name:    "poll_notification",
		Type:    models.NotificationTypePoll,
		Subject: "Новый опрос: {{.PollTitle}}",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Новый опрос</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #20c997; }
        .logo { font-size: 28px; font-weight: bold; color: #20c997; }
        .poll-icon { font-size: 48px; margin: 20px 0; }
        .poll-info { background-color: #f0fdfa; padding: 20px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #20c997; }
        .poll-title { font-size: 20px; color: #20c997; margin-bottom: 15px; font-weight: bold; }
        .poll-description { font-size: 16px; line-height: 1.6; margin-bottom: 15px; }
        .info-row { display: flex; justify-content: space-between; margin: 10px 0; padding: 5px 0; border-bottom: 1px solid #e0e0e0; }
        .info-label { font-weight: bold; color: #666; }
        .cta-button { display: inline-block; background-color: #20c997; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
            <div class="poll-icon">📊</div>
        </div>
        
        <div class="content">
            <h1>Новый опрос для вас!</h1>
            
            <div class="poll-info">
                <div class="poll-title">{{.PollTitle}}</div>
                
                {{if .PollDescription}}
                <div class="poll-description">{{.PollDescription}}</div>
                {{end}}
                
                <div class="info-row">
                    <span class="info-label">Тип опроса:</span>
                    <span>{{.PollType}}</span>
                </div>
                
                <div class="info-row">
                    <span class="info-label">Создал:</span>
                    <span>{{.CreatorName}}</span>
                </div>
                
                {{if .DeadlineDate}}
                <div class="info-row">
                    <span class="info-label">Срок участия:</span>
                    <span><strong>{{.DeadlineDate}}</strong></span>
                </div>
                {{end}}
                
                <div class="info-row">
                    <span class="info-label">Дата создания:</span>
                    <span>{{.CreatedAt}}</span>
                </div>
            </div>
            
            <div style="text-align: center;">
                <a href="{{.PollURL}}" class="cta-button">Принять участие</a>
            </div>
        </div>
        
        <div class="footer">
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
Новый опрос: {{.PollTitle}}

{{if .PollDescription}}Описание: {{.PollDescription}}{{end}}
Тип опроса: {{.PollType}}
Создал: {{.CreatorName}}
{{if .DeadlineDate}}Срок участия: {{.DeadlineDate}}{{end}}
Дата создания: {{.CreatedAt}}

Принять участие: {{.PollURL}}

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},

	"password_reset": {
		Name:    "password_reset",
		Type:    models.NotificationTypeSystem,
		Subject: "Сброс пароля в Tachyon Messenger",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Сброс пароля</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #dc3545; }
        .logo { font-size: 28px; font-weight: bold; color: #dc3545; }
        .security-icon { font-size: 48px; margin: 20px 0; }
        .content { padding: 30px 0; }
        .reset-info { background-color: #f8d7da; padding: 20px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #dc3545; }
        .reset-code { background-color: #f1f3f4; padding: 15px; text-align: center; font-size: 24px; font-weight: bold; font-family: monospace; border-radius: 5px; margin: 20px 0; border: 2px dashed #dc3545; }
        .cta-button { display: inline-block; background-color: #dc3545; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .warning { background-color: #fff3cd; padding: 15px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #ffc107; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
            <div class="security-icon">🔐</div>
        </div>
        
        <div class="content">
            <h1>Сброс пароля</h1>
            
            <p>Здравствуйте, {{.UserName}}!</p>
            
            <p>Мы получили запрос на сброс пароля для вашего аккаунта в Tachyon Messenger.</p>
            
            <div class="reset-info">
                <p><strong>Детали запроса:</strong></p>
                <p>Время запроса: {{.RequestTime}}</p>
                <p>IP-адрес: {{.RequestIP}}</p>
            </div>
            
            {{if .ResetCode}}
            <p>Используйте следующий код для сброса пароля:</p>
            <div class="reset-code">{{.ResetCode}}</div>
            <p>Код действителен в течение {{.CodeExpiration}} минут.</p>
            {{else}}
            <p>Чтобы сбросить пароль, нажмите на кнопку ниже:</p>
            <div style="text-align: center;">
                <a href="{{.ResetURL}}" class="cta-button">Сбросить пароль</a>
            </div>
            <p>Ссылка действительна в течение {{.LinkExpiration}} часов.</p>
            {{end}}
            
            <div class="warning">
                <p><strong>⚠️ Важная информация по безопасности:</strong></p>
                <ul>
                    <li>Если вы не запрашивали сброс пароля, просто проигнорируйте это письмо</li>
                    <li>Никогда не передавайте код сброса или ссылку третьим лицам</li>
                    <li>После сброса пароля рекомендуем использовать надежный пароль</li>
                </ul>
            </div>
        </div>
        
        <div class="footer">
            <p>Если у вас возникли проблемы, обратитесь в службу поддержки</p>
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
Сброс пароля в Tachyon Messenger

Здравствуйте, {{.UserName}}!

Мы получили запрос на сброс пароля для вашего аккаунта в Tachyon Messenger.

Детали запроса:
Время запроса: {{.RequestTime}}
IP-адрес: {{.RequestIP}}

{{if .ResetCode}}
Используйте следующий код для сброса пароля: {{.ResetCode}}
Код действителен в течение {{.CodeExpiration}} минут.
{{else}}
Для сброса пароля перейдите по ссылке: {{.ResetURL}}
Ссылка действительна в течение {{.LinkExpiration}} часов.
{{end}}

ВАЖНАЯ ИНФОРМАЦИЯ ПО БЕЗОПАСНОСТИ:
• Если вы не запрашивали сброс пароля, просто проигнорируйте это письмо
• Никогда не передавайте код сброса или ссылку третьим лицам
• После сброса пароля рекомендуем использовать надежный пароль

Если у вас возникли проблемы, обратитесь в службу поддержки.

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},

	"daily_digest": {
		Name:    "daily_digest",
		Type:    models.NotificationTypeSystem,
		Subject: "Ежедневная сводка за {{.Date}}",
		HTMLTemplate: `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Ежедневная сводка</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #333; background-color: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; padding: 20px; border-radius: 10px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #6c757d; }
        .logo { font-size: 28px; font-weight: bold; color: #6c757d; }
        .digest-icon { font-size: 48px; margin: 20px 0; }
        .section { margin: 25px 0; padding: 20px; border-radius: 5px; }
        .section-title { font-size: 18px; font-weight: bold; margin-bottom: 15px; color: #495057; }
        .messages-section { background-color: #e7f3ff; border-left: 4px solid #007bff; }
        .tasks-section { background-color: #f0fff4; border-left: 4px solid #28a745; }
        .calendar-section { background-color: #fff8e1; border-left: 4px solid #ffc107; }
        .stat-item { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
        .stat-label { color: #666; }
        .stat-value { font-weight: bold; color: #333; }
        .highlight { background-color: #fff3cd; padding: 10px; border-radius: 3px; margin: 10px 0; }
        .cta-button { display: inline-block; background-color: #6c757d; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Tachyon Messenger</div>
            <div class="digest-icon">📊</div>
        </div>
        
        <div class="content">
            <h1>Ежедневная сводка за {{.Date}}</h1>
            <p>Добро пожаловать в вашу ежедневную сводку, {{.UserName}}!</p>
            
            {{if .MessagesStats}}
            <div class="section messages-section">
                <div class="section-title">💬 Сообщения</div>
                <div class="stat-item">
                    <span class="stat-label">Новых сообщений:</span>
                    <span class="stat-value">{{.MessagesStats.NewMessages}}</span>
                </div>
                <div class="stat-item">
                    <span class="stat-label">Непрочитанных:</span>
                    <span class="stat-value">{{.MessagesStats.UnreadMessages}}</span>
                </div>
                <div class="stat-item">
                    <span class="stat-label">Активных чатов:</span>
                    <span class="stat-value">{{.MessagesStats.ActiveChats}}</span>
                </div>
            </div>
            {{end}}
            
            {{if .TasksStats}}
            <div class="section tasks-section">
                <div class="section-title">📋 Задачи</div>
                <div class="stat-item">
                    <span class="stat-label">Новых задач:</span>
                    <span class="stat-value">{{.TasksStats.NewTasks}}</span>
                </div>
                <div class="stat-item">
                    <span class="stat-label">Выполнено:</span>
                    <span class="stat-value">{{.TasksStats.CompletedTasks}}</span>
                </div>
                <div class="stat-item">
                    <span class="stat-label">Просрочено:</span>
                    <span class="stat-value">{{.TasksStats.OverdueTasks}}</span>
                </div>
                {{if .TasksStats.UpcomingDeadlines}}
                <div class="highlight">
                    <strong>⏰ Приближающиеся дедлайны:</strong> {{.TasksStats.UpcomingDeadlines}}
                </div>
                {{end}}
            </div>
            {{end}}
            
            {{if .CalendarStats}}
            <div class="section calendar-section">
                <div class="section-title">📅 События</div>
                <div class="stat-item">
                    <span class="stat-label">Событий сегодня:</span>
                    <span class="stat-value">{{.CalendarStats.TodayEvents}}</span>
                </div>
                <div class="stat-item">
                    <span class="stat-label">События завтра:</span>
                    <span class="stat-value">{{.CalendarStats.TomorrowEvents}}</span>
                </div>
                {{if .CalendarStats.NextEvent}}
                <div class="highlight">
                    <strong>📌 Следующее событие:</strong> {{.CalendarStats.NextEvent}}
                </div>
                {{end}}
            </div>
            {{end}}
            
            <div style="text-align: center;">
                <a href="{{.AppURL}}" class="cta-button">Открыть Tachyon Messenger</a>
            </div>
        </div>
        
        <div class="footer">
            <p>Чтобы изменить настройки дайджеста, перейдите в настройки уведомлений</p>
            <p>С уважением, команда Tachyon Messenger</p>
            <p>Это автоматическое сообщение, отвечать на него не нужно.</p>
        </div>
    </div>
</body>
</html>`,
		TextTemplate: `
Ежедневная сводка за {{.Date}}

Добро пожаловать в вашу ежедневную сводку, {{.UserName}}!

{{if .MessagesStats}}
💬 СООБЩЕНИЯ
Новых сообщений: {{.MessagesStats.NewMessages}}
Непрочитанных: {{.MessagesStats.UnreadMessages}}
Активных чатов: {{.MessagesStats.ActiveChats}}
{{end}}

{{if .TasksStats}}
📋 ЗАДАЧИ
Новых задач: {{.TasksStats.NewTasks}}
Выполнено: {{.TasksStats.CompletedTasks}}
Просрочено: {{.TasksStats.OverdueTasks}}
{{if .TasksStats.UpcomingDeadlines}}⏰ Приближающиеся дедлайны: {{.TasksStats.UpcomingDeadlines}}{{end}}
{{end}}

{{if .CalendarStats}}
📅 СОБЫТИЯ
Событий сегодня: {{.CalendarStats.TodayEvents}}
События завтра: {{.CalendarStats.TomorrowEvents}}
{{if .CalendarStats.NextEvent}}📌 Следующее событие: {{.CalendarStats.NextEvent}}{{end}}
{{end}}

Открыть Tachyon Messenger: {{.AppURL}}

Чтобы изменить настройки дайджеста, перейдите в настройки уведомлений.

С уважением, команда Tachyon Messenger
Это автоматическое сообщение, отвечать на него не нужно.`,
		IsActive: true,
	},
}

// TemplateLoader helps to load templates into EmailSender
type TemplateLoader struct {
	sender EmailSender
}

// NewTemplateLoader creates a new template loader
func NewTemplateLoader(sender EmailSender) *TemplateLoader {
	return &TemplateLoader{
		sender: sender,
	}
}

// LoadDefaultTemplates loads all default templates into the sender
func (tl *TemplateLoader) LoadDefaultTemplates() error {
	var errors []string

	for name, tmpl := range DefaultEmailTemplates {
		if err := tl.loadTemplate(tmpl); err != nil {
			errors = append(errors, fmt.Sprintf("failed to load template %s: %v", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("template loading errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// LoadTemplate loads a single template
func (tl *TemplateLoader) LoadTemplate(tmpl *models.EmailTemplate) error {
	return tl.loadTemplate(tmpl)
}

// loadTemplate loads a template into the sender
func (tl *TemplateLoader) loadTemplate(tmpl *models.EmailTemplate) error {
	// Validate template
	if err := tl.validateTemplate(tmpl); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	// Load template into sender
	if smtpSender, ok := tl.sender.(*smtpSender); ok {
		return smtpSender.LoadTemplate(tmpl)
	}

	return fmt.Errorf("sender does not support template loading")
}

// validateTemplate validates a template
func (tl *TemplateLoader) validateTemplate(tmpl *models.EmailTemplate) error {
	if tmpl == nil {
		return fmt.Errorf("template is required")
	}

	if strings.TrimSpace(tmpl.Name) == "" {
		return fmt.Errorf("template name is required")
	}

	if strings.TrimSpace(tmpl.Subject) == "" {
		return fmt.Errorf("template subject is required")
	}

	if tmpl.HTMLTemplate == "" && tmpl.TextTemplate == "" {
		return fmt.Errorf("either HTML or text template is required")
	}

	// Validate HTML template syntax if provided
	if tmpl.HTMLTemplate != "" {
		if _, err := template.New("test_html").Parse(tmpl.HTMLTemplate); err != nil {
			return fmt.Errorf("invalid HTML template syntax: %w", err)
		}
	}

	// Validate text template syntax if provided
	if tmpl.TextTemplate != "" {
		if _, err := template.New("test_text").Parse(tmpl.TextTemplate); err != nil {
			return fmt.Errorf("invalid text template syntax: %w", err)
		}
	}

	return nil
}

// GetTemplateVariables returns expected variables for a template
func GetTemplateVariables(templateName string) []string {
	switch templateName {
	case "welcome":
		return []string{"UserName", "AppURL"}
	case "task_assigned":
		return []string{"TaskTitle", "TaskDescription", "TaskPriority", "AssignerName", "DueDate", "CreatedAt", "TaskURL"}
	case "message_notification":
		return []string{"SenderName", "ChatName", "MessageContent", "CreatedAt", "ChatURL"}
	case "calendar_reminder":
		return []string{"EventTitle", "EventDescription", "StartTime", "EndTime", "Location", "Participants", "EventURL"}
	case "system_announcement":
		return []string{"AnnouncementTitle", "AnnouncementContent", "IsImportant", "ActionRequired", "ReadMoreURL", "PublishedAt"}
	case "poll_notification":
		return []string{"PollTitle", "PollDescription", "PollType", "CreatorName", "DeadlineDate", "CreatedAt", "PollURL"}
	case "password_reset":
		return []string{"UserName", "RequestTime", "RequestIP", "ResetCode", "CodeExpiration", "ResetURL", "LinkExpiration"}
	case "daily_digest":
		return []string{"Date", "UserName", "MessagesStats", "TasksStats", "CalendarStats", "AppURL"}
	default:
		return []string{}
	}
}

// GetTemplateDescription returns description for a template
func GetTemplateDescription(templateName string) string {
	descriptions := map[string]string{
		"welcome":              "Приветственное письмо для новых пользователей",
		"task_assigned":        "Уведомление о назначении новой задачи",
		"message_notification": "Уведомление о новом сообщении в чате",
		"calendar_reminder":    "Напоминание о предстоящем событии",
		"system_announcement":  "Системное объявление или важная информация",
		"poll_notification":    "Уведомление о новом опросе",
		"password_reset":       "Письмо для сброса пароля",
		"daily_digest":         "Ежедневная сводка активности",
	}

	if desc, exists := descriptions[templateName]; exists {
		return desc
	}
	return "Описание недоступно"
}
