package email

import (
	"bytes"
	"html/template"
)

// renderInvitationTemplate renders the invitation email template
func (s *EmailService) renderInvitationTemplate(userName, inviteToken, deepLink string) (string, error) {
	// Get app store links from environment
	appStoreURL := getEnvOrDefault("APP_STORE_URL", "https://apps.apple.com/app/tachyon-messenger")
	googlePlayURL := getEnvOrDefault("GOOGLE_PLAY_URL", "https://play.google.com/store/apps/details?id=com.tachyon.messenger")

	// Get backend URL for invitation redirect page
	backendURL := getEnvOrDefault("BACKEND_URL", getEnvOrDefault("USER_SERVICE_URL", "http://localhost:8081"))
	inviteURL := backendURL + "/invite/" + inviteToken

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Приглашение в Tachyon Messenger</title>
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
            color: #E94444;
            margin: 0;
            font-size: 28px;
        }
        .step {
            background-color: #f8f9fa;
            border-left: 4px solid #E94444;
            padding: 20px;
            margin: 20px 0;
            border-radius: 4px;
        }
        .step-number {
            display: inline-block;
            background-color: #E94444;
            color: white;
            width: 30px;
            height: 30px;
            border-radius: 50%;
            text-align: center;
            line-height: 30px;
            font-weight: bold;
            margin-right: 10px;
        }
        .step-title {
            font-size: 18px;
            font-weight: bold;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .app-links {
            display: flex;
            justify-content: center;
            gap: 15px;
            margin: 20px 0;
            flex-wrap: wrap;
        }
        .app-link {
            display: inline-block;
            padding: 0;
        }
        .app-link img {
            height: 50px;
            border-radius: 8px;
        }
        .button {
            display: inline-block;
            background-color: #E94444;
            color: #ffffff !important;
            padding: 15px 30px;
            text-decoration: none;
            border-radius: 8px;
            margin: 10px 5px;
            font-weight: bold;
            text-align: center;
            font-size: 16px;
        }
        .button:hover {
            background-color: #d93636;
        }
        .code-box {
            background-color: #fff;
            border: 2px dashed #E94444;
            border-radius: 8px;
            padding: 15px;
            text-align: center;
            margin: 15px 0;
        }
        .code {
            font-size: 24px;
            font-weight: bold;
            color: #E94444;
            letter-spacing: 2px;
            font-family: 'Courier New', monospace;
            word-break: break-all;
        }
        .info-box {
            background-color: #e7f3ff;
            border: 1px solid #b3d9ff;
            border-radius: 4px;
            padding: 15px;
            margin: 15px 0;
            color: #004085;
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
        @media (max-width: 600px) {
            .button {
                display: block;
                margin: 10px 0;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📱 Приглашение в Tachyon Messenger</h1>
        </div>

        <p style="text-align: center; font-size: 18px; color: #2c3e50;">
            Здравствуйте, <strong>{{.UserName}}</strong>!
        </p>

        <p style="text-align: center;">
            Вы приглашены присоединиться к корпоративному мессенджеру <strong>Tachyon</strong>.
        </p>

        <!-- Quick Action Button -->
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.InviteURL}}" class="button" target="_blank" rel="noopener">Принять приглашение</a>
            <p style="margin: 15px 0 0 0; font-size: 14px; color: #6c757d;">
                Нажмите кнопку, чтобы автоматически открыть приложение
            </p>
        </div>

        <!-- Fallback link for email clients that don't handle buttons properly -->
        <div style="background-color: #f8f9fa; border: 1px solid #dee2e6; border-radius: 8px; padding: 15px; margin: 20px 0;">
            <p style="margin: 0 0 10px 0; font-size: 14px; color: #2c3e50; text-align: center;">
                <strong>Или скопируйте ссылку:</strong>
            </p>
            <div style="background: white; padding: 10px; border: 1px solid #dee2e6; border-radius: 4px; word-break: break-all; text-align: center;">
                <a href="{{.InviteURL}}" style="color: #E94444; text-decoration: none; font-size: 13px;">{{.InviteURL}}</a>
            </div>
            <p style="margin: 10px 0 0 0; font-size: 12px; color: #6c757d; text-align: center;">
                Скопируйте и вставьте в браузер
            </p>
        </div>

        <div class="info-box">
            <strong>📱 Как это работает:</strong>
            <ul style="margin: 10px 0; padding-left: 20px;">
                <li>Нажмите на кнопку "Принять приглашение"</li>
                <li>Откроется страница, которая автоматически перенаправит вас в приложение</li>
                <li>Если приложение не установлено, вы получите инструкции и код приглашения</li>
            </ul>
        </div>

        <hr style="margin: 30px 0; border: none; border-top: 1px solid #e9ecef;" />

        <h3 style="text-align: center; color: #2c3e50; margin: 20px 0;">Или выполните шаги вручную:</h3>

        <!-- Step 1: Install App -->
        <div class="step">
            <div class="step-title">
                <span class="step-number">1</span>
                Установите приложение
            </div>
            <p style="margin: 10px 0 15px 40px;">
                Скачайте приложение Tachyon Messenger из App Store или Google Play:
            </p>
            <div class="app-links">
                <a href="{{.AppStoreURL}}" class="app-link" target="_blank" rel="noopener">
                    <img src="https://tools.applemediaservices.com/api/badges/download-on-the-app-store/black/en-us?size=250x83" alt="Download on App Store" />
                </a>
                <a href="{{.GooglePlayURL}}" class="app-link" target="_blank" rel="noopener">
                    <img src="https://play.google.com/intl/en_us/badges/static/images/badges/en_badge_web_generic.png" alt="Get it on Google Play" style="height: 58px;" />
                </a>
            </div>
        </div>

        <!-- Step 2: Copy Invitation Code -->
        <div class="step">
            <div class="step-title">
                <span class="step-number">2</span>
                Скопируйте код приглашения
            </div>
            <p style="margin: 10px 0 15px 40px;">
                Скопируйте код ниже - он понадобится для активации в приложении:
            </p>
            <div class="code-box">
                <div class="code">{{.InviteToken}}</div>
                <p style="margin: 10px 0 0 0; font-size: 13px; color: #6c757d;">
                    Нажмите и удерживайте, чтобы скопировать код
                </p>
            </div>
        </div>

        <!-- Step 3: Open App and Enter Code -->
        <div class="step">
            <div class="step-title">
                <span class="step-number">3</span>
                Откройте приложение и введите код
            </div>
            <div class="info-box">
                <strong>💡 Инструкция:</strong><br>
                1. Откройте приложение Tachyon Messenger<br>
                2. Нажмите "Есть приглашение?" или "У меня есть код"<br>
                3. Вставьте скопированный код приглашения
            </div>
        </div>

        <!-- Step 4: Create Password -->
        <div class="step">
            <div class="step-title">
                <span class="step-number">4</span>
                Придумайте пароль
            </div>
            <p style="margin: 10px 0 0 40px;">
                После ввода кода приглашения вам будет предложено создать надежный пароль для вашей учетной записи.
            </p>
        </div>

        <!-- Step 5: Login -->
        <div class="step">
            <div class="step-title">
                <span class="step-number">5</span>
                Авторизуйтесь в системе
            </div>
            <p style="margin: 10px 0 0 40px;">
                После создания пароля вы сможете авторизоваться используя вашу почту и новый пароль.
            </p>
        </div>

        <div class="warning">
            <strong>⚠️ Важно:</strong>
            <ul style="margin: 10px 0; padding-left: 20px;">
                <li>Никому не сообщайте код приглашения</li>
                <li>Используйте надежный пароль (минимум 8 символов)</li>
                <li>Если у вас возникли проблемы, свяжитесь с администратором</li>
            </ul>
        </div>

        <div class="footer">
            <p>С уважением,<br>Команда Tachyon Messenger</p>
            <p style="font-size: 12px; color: #adb5bd;">
                Это автоматическое сообщение. Если вы не ожидали этого приглашения, проигнорируйте его.
            </p>
        </div>
    </div>
</body>
</html>
`

	// Parse and execute template
	t, err := template.New("invitation").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := struct {
		UserName      string
		InviteToken   string
		DeepLink      string
		InviteURL     string
		AppStoreURL   string
		GooglePlayURL string
	}{
		UserName:      userName,
		InviteToken:   inviteToken,
		DeepLink:      deepLink,
		InviteURL:     inviteURL,
		AppStoreURL:   appStoreURL,
		GooglePlayURL: googlePlayURL,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
