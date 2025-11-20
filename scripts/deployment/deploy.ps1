# ==============================================
# Tachyon Deployment Script for Windows
# ==============================================
# PowerShell script for deploying Tachyon Messenger
# Uses GitHub Container Registry (ghcr.io)

param(
    [Parameter(Position=0)]
    [ValidateSet('deploy','pull','status','health','logs','down','restart')]
    [string]$Action = 'deploy',

    [Parameter(Position=1)]
    [string]$Service = ''
)

# Configuration
$COMPOSE_FILE = "docker-compose.prod.yml"
$ENV_FILE = ".env.production"
$VERSION_FILE = ".env.versions"

# Functions
function Write-Header {
    param([string]$Message)
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host $Message -ForegroundColor Blue
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host ""
}

function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-ErrorMsg {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

function Write-Warning {
    param([string]$Message)
    Write-Host "⚠ $Message" -ForegroundColor Yellow
}

function Write-Info {
    param([string]$Message)
    Write-Host "ℹ $Message" -ForegroundColor Cyan
}

# Check prerequisites
function Test-Prerequisites {
    Write-Header "Checking Prerequisites"

    # Check Docker
    try {
        $null = docker --version
        Write-Success "Docker is installed"
    } catch {
        Write-ErrorMsg "Docker is not installed"
        exit 1
    }

    # Check Docker Compose
    try {
        $null = docker compose version
        Write-Success "Docker Compose is installed"
    } catch {
        Write-ErrorMsg "Docker Compose is not installed"
        exit 1
    }

    # Check environment files
    if (-not (Test-Path $ENV_FILE)) {
        Write-ErrorMsg "Environment file $ENV_FILE not found"
        exit 1
    }
    Write-Success "Environment file found: $ENV_FILE"

    if (-not (Test-Path $VERSION_FILE)) {
        Write-Warning "Version file $VERSION_FILE not found, will use 'latest' tags"
    } else {
        Write-Success "Version file found: $VERSION_FILE"
    }
}

# Pull latest images
function Invoke-Pull {
    Write-Header "Pulling Latest Images"

    if (Test-Path $VERSION_FILE) {
        docker compose -f $COMPOSE_FILE --env-file $ENV_FILE --env-file $VERSION_FILE pull
    } else {
        docker compose -f $COMPOSE_FILE --env-file $ENV_FILE pull
    }

    if ($LASTEXITCODE -eq 0) {
        Write-Success "Images pulled successfully"
    } else {
        Write-ErrorMsg "Failed to pull images"
        exit 1
    }
}

# Deploy services
function Invoke-Deploy {
    Write-Header "Deploying Services"

    if (Test-Path $VERSION_FILE) {
        docker compose -f $COMPOSE_FILE --env-file $ENV_FILE --env-file $VERSION_FILE up -d
    } else {
        docker compose -f $COMPOSE_FILE --env-file $ENV_FILE up -d
    }

    if ($LASTEXITCODE -eq 0) {
        Write-Success "Services deployed successfully"
    } else {
        Write-ErrorMsg "Failed to deploy services"
        exit 1
    }
}

# Health check
function Test-Health {
    Write-Header "Running Health Checks"

    Start-Sleep -Seconds 10

    $services = @(
        @{Name="tachyon-gateway-prod"; Port=8080},
        @{Name="tachyon-user-service-prod"; Port=8081},
        @{Name="tachyon-chat-service-prod"; Port=8082},
        @{Name="tachyon-task-service-prod"; Port=8083},
        @{Name="tachyon-calendar-service-prod"; Port=8084},
        @{Name="tachyon-poll-service-prod"; Port=8085},
        @{Name="tachyon-analytics-service-prod"; Port=8086},
        @{Name="tachyon-notification-service-prod"; Port=8087},
        @{Name="tachyon-file-service-prod"; Port=8088},
        @{Name="tachyon-backup-service-prod"; Port=8089}
    )

    $allHealthy = $true

    foreach ($service in $services) {
        $containerName = $service.Name
        $port = $service.Port

        $running = docker ps --format "{{.Names}}" | Select-String -Pattern "^$containerName$"

        if ($running) {
            try {
                $null = docker exec $containerName wget --no-verbose --tries=1 --spider "http://localhost:$port/health" 2>$null
                if ($LASTEXITCODE -eq 0) {
                    Write-Success "$containerName is healthy"
                } else {
                    Write-ErrorMsg "$containerName health check failed"
                    $allHealthy = $false
                }
            } catch {
                Write-ErrorMsg "$containerName health check failed"
                $allHealthy = $false
            }
        } else {
            Write-ErrorMsg "$containerName is not running"
            $allHealthy = $false
        }
    }

    if ($allHealthy) {
        Write-Success "All services are healthy!"
    } else {
        Write-ErrorMsg "Some services are not healthy. Check logs with: docker compose -f $COMPOSE_FILE logs"
        exit 1
    }
}

# Show running services
function Show-Status {
    Write-Header "Service Status"
    docker compose -f $COMPOSE_FILE ps
}

# Main execution
function Invoke-Main {
    Write-Header "Tachyon Production Deployment"

    Test-Prerequisites
    Invoke-Pull
    Invoke-Deploy
    Test-Health
    Show-Status

    Write-Header "Deployment Complete!"
    Write-Success "All services are running and healthy"
    Write-Info "Gateway is available at: http://localhost:8080"
}

# Execute action
switch ($Action) {
    'deploy' {
        Invoke-Main
    }
    'pull' {
        Test-Prerequisites
        Invoke-Pull
    }
    'status' {
        Show-Status
    }
    'health' {
        Test-Health
    }
    'logs' {
        if ($Service) {
            docker compose -f $COMPOSE_FILE logs -f $Service
        } else {
            docker compose -f $COMPOSE_FILE logs -f
        }
    }
    'down' {
        Write-Header "Stopping Services"
        docker compose -f $COMPOSE_FILE down
        Write-Success "Services stopped"
    }
    'restart' {
        Write-Header "Restarting Services"
        if ($Service) {
            docker compose -f $COMPOSE_FILE restart $Service
        } else {
            docker compose -f $COMPOSE_FILE restart
        }
        Write-Success "Services restarted"
    }
}
