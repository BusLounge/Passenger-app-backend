# Choreo Deployment Guide - SMS Authentication Backend

Complete step-by-step guide to deploy your Go backend to WSO2 Choreo with Docker.

---

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Pre-Deployment Checklist](#pre-deployment-checklist)
3. [Step 1: Prepare Your GitHub Repository](#step-1-prepare-your-github-repository)
4. [Step 2: Set Up Choreo Account](#step-2-set-up-choreo-account)
5. [Step 3: Create a New Component](#step-3-create-a-new-component)
6. [Step 4: Configure Environment Variables](#step-4-configure-environment-variables)
7. [Step 5: Build and Test Locally](#step-5-build-and-test-locally)
8. [Step 6: Deploy to Choreo](#step-6-deploy-to-choreo)
9. [Step 7: Monitor and Verify](#step-7-monitor-and-verify)
10. [Step 8: Configure Custom Domain (Optional)](#step-8-configure-custom-domain-optional)
11. [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before deploying to Choreo, ensure you have:

- ✅ A GitHub account with your repository
- ✅ WSO2 Choreo account (free tier available at [choreo.dev](https://choreo.dev))
- ✅ Docker installed locally (for testing)
- ✅ Go 1.24+ installed locally
- ✅ Git installed and configured

### Quick Setup

```bash
# Check Docker
docker --version

# Check Go
go version

# Check Git
git --version
```

---

## Pre-Deployment Checklist

Before deploying, verify:

- [ ] All sensitive data is in `.env` files, NOT in code
- [ ] `.env` file is in `.gitignore` (review: `backend/.gitignore`)
- [ ] `.env.example` contains all required variables (non-secrets)
- [ ] Dockerfile exists at `backend/Dockerfile`
- [ ] `.choreo/component.yaml` is configured correctly
- [ ] All tests pass: `make test`
- [ ] Docker image builds successfully locally
- [ ] Health check endpoint is working: `/health`
- [ ] API documentation is updated in `swagger.yaml`

### Verification Commands

```bash
# From backend directory
cd backend

# Run tests
make test

# Build Docker image locally
make docker-build

# Run container locally
make docker-run

# Test the container
# In another terminal:
curl http://localhost:8080/health
```

---

## Step 1: Prepare Your GitHub Repository

### 1.1 Ensure Your Code is Pushed to GitHub

```bash
# From project root
cd c:\myProjects\AASL\PassengerApp2

# Check git status
git status

# Add changes
git add .

# Commit
git commit -m "Prepare backend for Choreo deployment"

# Push to GitHub
git push origin main
```

### 1.2 Create `.env.example` (Already Done)

Verify that `backend/.env.example` exists with all required variables:

```bash
DATABASE_URL=postgresql://user:password@host:port/database
JWT_SECRET=your-secret-key
JWT_REFRESH_SECRET=your-refresh-secret-key
JWT_ACCESS_TOKEN_EXPIRY=3600
JWT_REFRESH_TOKEN_EXPIRY=604800
SMS_MODE=production
DIALOG_SMS_METHOD=url
DIALOG_SMS_ESMSQK=your-esmsqk-key
DIALOG_SMS_MASK=YourMask
```

### 1.3 Verify .gitignore

Ensure `.env` is in `.gitignore`:

```bash
# Check if .env is ignored
cat backend/.gitignore | grep "\.env"
```

Expected output should include:
```
.env
.env.prod
```

---

## Step 2: Set Up Choreo Account

### 2.1 Create an Account

1. Visit [https://choreo.dev](https://choreo.dev)
2. Click **Sign Up** → Register with email or GitHub
3. Complete email verification
4. Create an organization (if first time)

### 2.2 Get Your Organization ID

1. Log in to [Choreo Console](https://console.choreo.dev)
2. Navigate to **Settings** → **Organization**
3. Copy your **Organization ID**

### 2.3 Create a Project

1. Click **New Project**
2. Enter project name: `SmartTransit-Backend`
3. Click **Create**

---

## Step 3: Create a New Component

### Option A: Using Choreo Console (Recommended for First Time)

1. In your project, click **+ New Component**
2. Select **Dockerfile**
3. Choose **GitHub** as source
4. Authorize Choreo to access your GitHub
5. Select your repository: `AASL/PassengerApp2`
6. Select branch: `main`
7. Enter **Dockerfile Path**: `backend/Dockerfile`
8. Component name: `sms-auth-backend`
9. Click **Create**

### Option B: Using Git Webhook (Recommended for CI/CD)

1. Choreo will auto-detect the `.choreo/component.yaml` file
2. Component is registered automatically on push
3. Build starts automatically

---

## Step 4: Configure Environment Variables

### 4.1 In Choreo Console

1. Go to your component: **sms-auth-backend**
2. Click **Deploy** → **Configurations**
3. Click **+ Add Config**
4. For each environment variable from `.env.example`, add:

| Variable | Value | Type |
|----------|-------|------|
| `DATABASE_URL` | Your Supabase PostgreSQL URL | Secure |
| `JWT_SECRET` | 64-character random string | Secure |
| `JWT_REFRESH_SECRET` | 64-character random string | Secure |
| `JWT_ACCESS_TOKEN_EXPIRY` | `3600` | Non-secure |
| `JWT_REFRESH_TOKEN_EXPIRY` | `604800` | Non-secure |
| `SMS_MODE` | `production` | Non-secure |
| `DIALOG_SMS_METHOD` | `url` | Non-secure |
| `DIALOG_SMS_ESMSQK` | Your Dialog eSMS key | Secure |
| `DIALOG_SMS_MASK` | Your SMS mask | Secure |

### 4.2 Setting Up Database

If using Supabase:

```bash
# 1. Create PostgreSQL instance at https://supabase.com
# 2. Get connection string:
# - Go to Settings → Database → Connection String
# - Copy PostgreSQL URL
# - Should look like:
postgresql://postgres:YourPassword@db.YOUR-PROJECT.supabase.co:5432/postgres
```

### 4.3 Generating JWT Secrets

```powershell
# PowerShell - Generate secure random strings
$secret = [Convert]::ToBase64String((0..31 | ForEach-Object { [byte](Get-Random -Minimum 0 -Maximum 256) }))
Write-Output $secret

# Do this twice, once for JWT_SECRET and once for JWT_REFRESH_SECRET
```

---

## Step 5: Build and Test Locally

### 5.1 Create `.env` File Locally

```bash
# From backend directory
cp .env.example .env

# Edit .env with your actual values
# On Windows: use Notepad or VS Code
code .env
```

### 5.2 Build Docker Image

```bash
cd backend

# Build the Docker image
make docker-build

# Or manually:
docker build -t sms-auth-backend:latest .
```

### 5.3 Run Container Locally

```bash
# Run the Docker container
make docker-run

# Or manually:
docker run --env-file .env -p 8080:8080 sms-auth-backend:latest
```

### 5.4 Test the Container

```powershell
# In a new terminal/PowerShell
# Test health endpoint
Invoke-WebRequest -Uri "http://localhost:8080/health" -Method Get

# Or using curl (if installed)
curl http://localhost:8080/health

# Test a sample endpoint
Invoke-WebRequest -Uri "http://localhost:8080/api/ping" -Method Get
```

### 5.5 Stop Container

```bash
# Find container ID
docker ps

# Stop the container
docker stop <CONTAINER_ID>
```

---

## Step 6: Deploy to Choreo

### 6.1 Trigger Build Automatically

**Option 1: Push to GitHub (Automatic)**

```bash
cd c:\myProjects\AASL\PassengerApp2

git add .
git commit -m "Deploy backend to Choreo"
git push origin main
```

Choreo will:
- Detect the push
- Build the Docker image
- Run tests
- Deploy to staging/production

### 6.2 Manual Build in Choreo

1. Go to **Choreo Console**
2. Select your component: **sms-auth-backend**
3. Click **Build & Deploy**
4. Select **Build** tab
5. Click **Build**
6. Monitor build logs

### 6.3 Monitor Build Progress

1. Click **Builds** Tab
2. Click on the latest build
3. View **Build Logs**:
   - Docker image compilation
   - Dependency download
   - Build success confirmation

---

## Step 7: Monitor and Verify

### 7.1 Check Deployment Status

1. Go to **Deployments** tab
2. Find your latest deployment
3. Status should be **Active** (green)

### 7.2 Get Your Service URL

1. Click on the deployment
2. Find **Service URL** or **Endpoint**
3. Format: `https://sms-auth-api-xxx.c1.choreo.dev`

### 7.3 Test Your Deployed API

```powershell
# Get the service URL from Choreo console
$serviceUrl = "https://sms-auth-api-858.c1.choreo.dev"

# Test health endpoint
Invoke-WebRequest -Uri "$serviceUrl/health" -Method Get

# Test with curl
curl "$serviceUrl/health"

# Test API endpoints
Invoke-WebRequest -Uri "$serviceUrl/api/auth/login" -Method Post `
  -Headers @{"Content-Type"="application/json"} `
  -Body '{"phone":"+94712345678","password":"test123"}'
```

### 7.4 View Logs

1. Select your deployment
2. Click **Logs**
3. View real-time logs:
   - Application logs
   - Error logs
   - Access logs

### 7.5 Health Check Verification

1. Choreo automatically performs health checks
2. Check the health endpoint: GET `/health`
3. Response should be: `{"status":"ok"}`

---

## Step 8: Configure Custom Domain (Optional)

### 8.1 Add Custom Domain

1. Go to **Settings** → **Domain**
2. Click **+ Add Domain**
3. Enter your domain: `api.yourdomain.com`
4. Add DNS records (provided by Choreo)
5. Click **Verify**

### 8.2 Update API Documentation

Update frontend/clients with new URL:

```
Old: https://sms-auth-api-858.c1.choreo.dev
New: https://api.yourdomain.com
```

---

## Troubleshooting

### Issue 1: Build Fails

**Symptom**: Red build status

**Solution**:
1. Check **Build Logs** tab
2. Common causes:
   - Missing `go.mod` or `go.sum`
   - Dockerfile syntax error
   - Port conflicts

```bash
# Fix locally first
cd backend
make build           # Check if local build works
make docker-build    # Check Docker build
```

### Issue 2: Application Crashes on Startup

**Symptom**: Deployment Active but no response

**Solution**:
1. Check **Logs** tab
2. Look for database connection errors:
   ```
   "Failed to connect to database"
   ```
3. Verify `DATABASE_URL` environment variable
4. Ensure database is accessible from Choreo IPs

### Issue 3: Health Check Fails

**Symptom**: `UNHEALTHY` status

**Solution**:
1. Verify `/health` endpoint exists in code
2. Check if it's protected by middleware
3. Should return without authentication
4. Response should be plain text or JSON

```go
// Example health endpoint
router.GET("/health", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
})
```

### Issue 4: Environment Variables Not Loading

**Symptom**: Application throws "ENV variable not found"

**Solution**:
1. Verify variable name: **case-sensitive**
2. Check **Configurations** tab in Choreo
3. Redeploy after adding variables
4. Check logs for actual values being used

### Issue 5: Database Connection Timeout

**Symptom**: All requests return 500 error

**Solution**:
1. Test database connectivity:
   ```bash
   # From your local machine
   postgresql://user:pass@db.supabase.co:5432/postgres
   ```
2. Verify PostgreSQL port (default 5432)
3. Check if whitelist IP addresses (Choreo IPs need access)
4. For Supabase: Settings → Database → Connection pooling

### Issue 6: Docker Image Too Large

**Symptom**: Build takes too long or fails at "pushing"

**Solution**:
- Current setup uses multi-stage build (optimized)
- Check if unnecessary files in `.dockerignore`

```dockerfile
# Optimize .dockerignore
.git
.gitignore
node_modules
.env
.DS_Store
*.log
build/
dist/
```

---

## Performance Optimization

### 1. Database Connection Pooling

```go
// In config.go
db.SetMaxOpenConns(10)        // Reduce from default
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

### 2. Response Caching

Add cache headers in middleware:
```go
c.Header("Cache-Control", "public, max-age=300")
```

### 3. Request Timeout

```go
// Set timeouts
server := &http.Server{
    Addr:           ":8080",
    Handler:        router,
    ReadTimeout:    15 * time.Second,
    WriteTimeout:   15 * time.Second,
    MaxHeaderBytes: 1 << 20,
}
```

---

## Security Checklist

- [ ] No hardcoded secrets in code
- [ ] All secrets in environment variables (marked as Secure in Choreo)
- [ ] CORS properly configured
- [ ] JWT tokens have proper expiry
- [ ] Database credentials rotated
- [ ] HTTPS enabled (automatic with Choreo)
- [ ] Rate limiting configured
- [ ] Input validation on all endpoints
- [ ] Logs don't contain sensitive data

---

## Monitoring & Alerts (Optional)

### Set Up Monitoring

1. Go to **Monitoring** tab
2. View metrics:
   - CPU usage
   - Memory usage
   - Request count
   - Error rate

### Set Up Alerts

1. Click **+ Add Alert**
2. Configure thresholds:
   - CPU > 80%
   - Memory > 512MB
   - Error rate > 5%

---

## Useful Commands

```bash
# View Dockerfile
cat backend/Dockerfile

# Build locally
cd backend && make docker-build

# Run locally
make docker-run

# Push to GitHub
git push origin main

# View environment variables
cat backend/.env.example

# Generate random JWT secret
$secret = [Convert]::ToBase64String((0..63 | ForEach-Object { [byte](Get-Random -Minimum 0 -Maximum 256) }))
Write-Output $secret
```

---

## Support & Resources

- **Choreo Documentation**: https://choreo.dev/docs
- **Docker Documentation**: https://docs.docker.com
- **Go Documentation**: https://golang.org/doc
- **Supabase Documentation**: https://supabase.com/docs
- **WSO2 Community**: https://discord.gg/wso2

---

## Summary

Your deployment is complete when:

✅ Build shows **COMPLETE** status  
✅ Deployment shows **ACTIVE** status  
✅ Health endpoint returns `{"status":"ok"}`  
✅ API endpoints respond correctly  
✅ Logs show no errors  
✅ Performance metrics are healthy  

**Your API is now live on Choreo! 🚀**

---

**Last Updated**: March 8, 2026  
**Backend Version**: 1.0.0  
**Go Version**: 1.24+
