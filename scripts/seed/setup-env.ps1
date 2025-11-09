# Скрипт для переключения .env между Docker и локальным режимом
# Использование: .\setup-env.ps1 [docker|local]

param(
    [Parameter(Mandatory=$true)]
    [ValidateSet("docker", "local")]
    [string]$Mode
)

$envFile = "..\..\env"

if ($Mode -eq "local") {
    Write-Host "🔧 Настройка .env для локального запуска скриптов..." -ForegroundColor Cyan

    # Заменяем postgres -> localhost
    (Get-Content $envFile) -replace '@postgres:', '@localhost:' `
                           -replace '@redis:', '@localhost:' |
        Set-Content $envFile

    Write-Host "✅ .env настроен для локального запуска (localhost)" -ForegroundColor Green
    Write-Host ""
    Write-Host "Теперь запустите PostgreSQL и Redis в Docker:" -ForegroundColor Yellow
    Write-Host "  docker compose up -d postgres redis" -ForegroundColor White
    Write-Host ""
    Write-Host "Затем запустите скрипт seeding:" -ForegroundColor Yellow
    Write-Host "  go run scripts/seed/main.go --all --clean" -ForegroundColor White

} elseif ($Mode -eq "docker") {
    Write-Host "🐳 Настройка .env для Docker..." -ForegroundColor Cyan

    # Заменяем localhost -> postgres/redis
    (Get-Content $envFile) -replace '@localhost:5432', '@postgres:5432' `
                           -replace '@localhost:6379', '@redis:6379' |
        Set-Content $envFile

    Write-Host "✅ .env настроен для Docker (postgres/redis)" -ForegroundColor Green
    Write-Host ""
    Write-Host "Теперь можно запускать приложение в Docker:" -ForegroundColor Yellow
    Write-Host "  docker compose up" -ForegroundColor White
}
