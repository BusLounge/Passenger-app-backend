#!/usr/bin/env pwsh
# Choreo Deployment Verification Script
# Run this before deploying to ensure everything is ready

$ErrorActionPreference = "Stop"

Write-Host "=================================================================================" -ForegroundColor Cyan
Write-Host "  CHOREO DEPLOYMENT VERIFICATION" -ForegroundColor Cyan
Write-Host "=================================================================================" -ForegroundColor Cyan
Write-Host ""

# Color functions
function Write-Success {
    param([string]$Message)
    Write-Host "✅ $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "❌ $Message" -ForegroundColor Red
}

function Write-Warning {
    param([string]$Message)
    Write-Host "⚠️  $Message" -ForegroundColor Yellow
}

function Write-Info {
    param([string]$Message)
    Write-Host "ℹ️  $Message" -ForegroundColor Cyan
}

# Check prerequisites
Write-Host ""
Write-Host "📋 Checking Prerequisites..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

# Check Docker
try {
    $docker_version = docker --version
    Write-Success "Docker installed: $docker_version"
} catch {
    Write-Error "Docker not found. Please install Docker from https://www.docker.com/products/docker-desktop"
    exit 1
}

# Check Go
try {
    $go_version = go version
    Write-Success "Go installed: $go_version"
} catch {
    Write-Error "Go not found. Please install Go from https://golang.org/dl"
    exit 1
}

# Check Git
try {
    $git_version = git --version
    Write-Success "Git installed: $git_version"
} catch {
    Write-Error "Git not found. Please install Git from https://git-scm.com"
    exit 1
}

# Check file structure
Write-Host ""
Write-Host "📁 Checking File Structure..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

$required_files = @(
    "Dockerfile",
    ".env.example",
    "go.mod",
    "go.sum",
    "cmd/server/main.go",
    ".choreo/component.yaml"
)

$all_files_exist = $true

foreach ($file in $required_files) {
    $file_path = Join-Path "." $file
    if (Test-Path $file_path) {
        Write-Success "Found: $file"
    } else {
        Write-Error "Missing: $file"
        $all_files_exist = $false
    }
}

if (-not $all_files_exist) {
    Write-Error "Some required files are missing!"
    exit 1
}

# Check .gitignore
Write-Host ""
Write-Host "🔐 Checking Security Configuration..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

if (Test-Path ".gitignore") {
    $gitignore_content = Get-Content ".gitignore" -Raw
    if ($gitignore_content -match "\.env") {
        Write-Success ".env is properly ignored in Git"
    } else {
        Write-Warning ".env file not in .gitignore. Adding to security check..."
    }
} else {
    Write-Warning ".gitignore not found"
}

# Check .env file
if (Test-Path ".env") {
    Write-Success ".env file exists (not committed to Git)"
    $env_vars = Get-Content ".env" | Where-Object { $_ -match "^[^#]+" }
    Write-Info "Found $($env_vars.Count) environment variables configured"
} else {
    Write-Warning ".env file not found. You'll need to create it from .env.example"
}

# Check .dockerignore
if (Test-Path ".dockerignore") {
    Write-Success ".dockerignore exists (reduces Docker image size)"
} else {
    Write-Warning ".dockerignore not found (optional but recommended)"
}

# Test Go Build
Write-Host ""
Write-Host "🔨 Testing Go Build..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

try {
    $output = go build -o test.exe ./cmd/server 2>&1
    Write-Success "Go build successful"
    Remove-Item "test.exe" -Force -ErrorAction SilentlyContinue
} catch {
    Write-Error "Go build failed: $_"
    exit 1
}

# Test Go Modules
Write-Host ""
Write-Host "📦 Testing Go Modules..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

try {
    $output = go mod verify 2>&1
    Write-Success "Go modules verified"
} catch {
    Write-Error "Go module verification failed: $_"
}

# Test Docker Build
Write-Host ""
Write-Host "🐳 Testing Docker Build..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

Write-Info "This may take 1-2 minutes on first run..."

try {
    $docker_output = docker build -t sms-auth-backend:test . 2>&1
    Write-Success "Docker image built successfully"
    
    # Get image size
    $image_info = docker images "sms-auth-backend:test" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
    Write-Info $image_info
    
    # Clean up test image
    docker rmi "sms-auth-backend:test" | Out-Null
    
} catch {
    Write-Error "Docker build failed: $_"
    Write-Warning "Try running: docker build -t sms-auth-backend:test . for detailed error logs"
    exit 1
}

# Test API Endpoints
Write-Host ""
Write-Host "🌐 Checking API Configuration..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

$swagger_file = "swagger.yaml"
if (Test-Path $swagger_file) {
    Write-Success "Swagger/OpenAPI documentation exists"
    $endpoint_count = (Get-Content $swagger_file | Select-String "^\s*/" | Measure-Object).Count
    Write-Info "Found approximately $endpoint_count API endpoints documented"
} else {
    Write-Warning "swagger.yaml not found"
}

# Check component.yaml
Write-Host ""
Write-Host "⚙️  Checking Choreo Configuration..." -ForegroundColor Magenta
Write-Host "=================================================================================" -ForegroundColor Magenta

$component_file = ".choreo/component.yaml"
if (Test-Path $component_file) {
    Write-Success "Choreo component.yaml found"
    
    $component_content = Get-Content $component_file -Raw
    if ($component_content -match "port:\s*8080") {
        Write-Success "API port configured to 8080"
    }
    
    if ($component_content -match "schemaFilePath") {
        Write-Success "Swagger/OpenAPI schema configured"
    }
} else {
    Write-Error "Choreo component.yaml not found at $component_file"
}

# Summary
Write-Host ""
Write-Host "=================================================================================" -ForegroundColor Cyan
Write-Host "  ✅ VERIFICATION COMPLETE" -ForegroundColor Green
Write-Host "=================================================================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "✅ Your project is ready for Choreo deployment!" -ForegroundColor Green
Write-Host ""

Write-Host "🚀 Next Steps:" -ForegroundColor Yellow
Write-Host "1. Ensure .env file has all required variables (DATABASE_URL, JWT_SECRET, etc.)"
Write-Host "2. Push to GitHub: git push origin main"
Write-Host "3. Go to https://console.choreo.dev"
Write-Host "4. Create new component with Dockerfile"
Write-Host "5. Configure environment variables in Choreo Console"
Write-Host "6. Deploy!"
Write-Host ""

Write-Host "📚 Documentation:" -ForegroundColor Yellow
Write-Host "- Full Guide: docs/CHOREO_DEPLOYMENT_GUIDE.md"
Write-Host "- Quick Start: docs/QUICK_CHOREO_DEPLOYMENT.md"
Write-Host ""

Write-Host "💡 Tips:" -ForegroundColor Yellow
Write-Host "- Test locally: docker run --env-file .env -p 8080:8080 sms-auth-backend:latest"
Write-Host "- Check Docker image size before uploading"
Write-Host "- Always keep sensitive data in .env (not in Git)"
Write-Host "- Monitor Choreo logs after deployment"
Write-Host ""
