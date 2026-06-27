# Quick Start: Deploy Backend to Choreo in 15 Minutes ⚡

## Prerequisites Check ✅

```powershell
# 1. Verify tools installed
docker --version
go version
git --version

# 2. Navigate to backend
cd c:\myProjects\AASL\PassengerApp2\backend

# 3. Install dependencies
go mod download

# 4. Run tests locally
go test ./...
```

## 5-Step Deployment Process

### Step 1: Prepare Local Build (2 min)
```powershell
cd backend

# Copy environment template
cp .env.example .env

# Edit .env with your database URL and secrets
code .env
```

**Required values in `.env`:**
- `DATABASE_URL` - Your PostgreSQL connection string
- `JWT_SECRET` - 64-character random string  
- `JWT_REFRESH_SECRET` - 64-character random string
- `DIALOG_SMS_ESMSQK` - Your Dialog SMS key

### Step 2: Test Docker Locally (3 min)
```powershell
# Build Docker image
docker build -t sms-auth-backend:latest .

# Run container
docker run --env-file .env -p 8080:8080 sms-auth-backend:latest

# In another terminal, test
curl http://localhost:8080/health
# Should return: {"status":"ok"}

# Stop container
docker stop <container_id>
```

### Step 3: Push to GitHub (1 min)
```powershell
cd c:\myProjects\AASL\PassengerApp2

git add .
git commit -m "Deploy backend to Choreo"
git push origin main
```

### Step 4: Create Choreo Component (5 min)

1. Go to https://console.choreo.dev
2. Sign in with GitHub account
3. Create project: `SmartTransit-Backend`
4. Click **+ New Component**
5. Select **Dockerfile** type
6. Connect GitHub → Select `AASL/PassengerApp2` repo
7. Enter **Dockerfile Path**: `backend/Dockerfile`
8. Name: `sms-auth-backend`
9. Click **Create**

### Step 5: Configure & Deploy (4 min)

1. Go to **Deploy** → **Configurations**
2. Add environment variables:
   - `DATABASE_URL` (Mark as Secure) ← Your Supabase URL
   - `JWT_SECRET` (Mark as Secure)
   - `JWT_REFRESH_SECRET` (Mark as Secure)  
   - `JWT_ACCESS_TOKEN_EXPIRY` = `3600`
   - `JWT_REFRESH_TOKEN_EXPIRY` = `604800`
   - `SMS_MODE` = `production`
   - `DIALOG_SMS_METHOD` = `url`
   - `DIALOG_SMS_ESMSQK` (Mark as Secure)
   - `DIALOG_SMS_MASK` = Your mask

3. Click **Build & Deploy**
4. Monitor build logs (takes ~2-3 min)
5. Wait for status: **ACTIVE** ✅

## Verify Deployment ✅

```powershell
# Get your service URL from Choreo Console
$serviceUrl = "https://sms-auth-api-xxx.c1.choreo.dev"

# Test health
Invoke-WebRequest -Uri "$serviceUrl/health"

# Test API
Invoke-WebRequest -Uri "$serviceUrl/api/ping"
```

## ✅ Success Indicators

- Build Status: **COMPLETE** 🟢
- Deployment Status: **ACTIVE** 🟢  
- Health Endpoint: Returns `{"status":"ok"}` ✅
- API Responding: No 502/503 errors ✅
- Logs: No ERROR level messages ✅

### Your API is live! 🚀
**URL Format**: `https://sms-auth-api-xxx.c1.choreo.dev`

---

## Common Issues & Quick Fixes

| Issue | Fix |
|-------|-----|
| Build fails | Check Build Logs tab, verify go.mod exists |
| App crashes | Missing DATABASE_URL variable, check Logs tab |
| Health check fails | Ensure /health endpoint exists and is public |
| Connection timeout | Database credentials wrong, check Supabase |
| Port 8080 conflict | Change port in component.yaml |

---

## Next Steps After Deployment

- [ ] Configure custom domain (optional)
- [ ] Set up monitoring & alerts
- [ ] Enable SSL/TLS (automatic with Choreo)
- [ ] Configure rate limiting
- [ ] Set up logging & debugging
- [ ] Add CI/CD pipeline
- [ ] Document API endpoints

---

## Useful Links

- Choreo Docs: https://choreo.dev/docs
- Dockerfile Reference: https://docs.docker.com/engine/reference/builder/
- Full Guide: See `CHOREO_DEPLOYMENT_GUIDE.md`

---

**Estimated Total Time**: 15 minutes ⏱️  
**Complexity**: Easy 🟢  
**Status**: Ready for Production  
