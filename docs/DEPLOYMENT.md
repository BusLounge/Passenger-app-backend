# Deployment Guide

Instructions for deploying the SmartTransit backend to different environments.

## Deployment Environments

| Environment | Purpose | Audience | Database |
|-------------|---------|----------|----------|
| **Development** | Local development | Developers | Local PostgreSQL |
| **Staging** | Testing & QA | QA Team | Staging DB |
| **Production** | Live application | End Users | Production DB |

## Pre-Deployment Checklist

- [ ] All tests pass: `go test ./...`
- [ ] Code linted: `golangci-lint run`
- [ ] Database migrations tested
- [ ] Environment variables configured
- [ ] SSL certificates (if HTTPS required)
- [ ] Backup of production database created
- [ ] Deployment rollback plan documented
- [ ] Health check endpoint verified

## Building for Deployment

### 1. Build Binary

**Production Build:**
```bash
# Build with optimizations
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-X main.version=1.0.0 -X main.buildTime=$(date)" \
    -o backend cmd/server/main.go
```

**Cross-Platform:**
```bash
# For Linux (from Windows/Mac)
GOOS=linux GOARCH=amd64 go build -o backend-linux cmd/server/main.go

# For Windows
GOOS=windows GOARCH=amd64 go build -o backend.exe cmd/server/main.go

# For macOS
GOOS=darwin GOARCH=amd64 go build -o backend-macos cmd/server/main.go
```

### 2. Test Binary

```bash
# Run and verify
./backend &

# Test health endpoint
curl http://localhost:8080/health

# Check logs for errors
# Should see: "Starting SmartTransit SMS Authentication Backend"
```

## Docker Deployment

### Building Docker Image

**Production Dockerfile:**
```dockerfile
# Already provided in project root
docker build -t smarttransit:1.0.0 .
```

**Tag for Registry:**
```bash
# Tag for Docker Hub
docker tag smarttransit:1.0.0 yourdockerhub/smarttransit:1.0.0
docker tag smarttransit:1.0.0 yourdockerhub/smarttransit:latest

# Tag for private registry
docker tag smarttransit:1.0.0 registry.company.com/smarttransit:1.0.0
```

### Running Container

**Basic Run:**
```bash
docker run -p 8080:8080 --env-file .env smarttransit:1.0.0
```

**Production Run (with resources limits):**
```bash
docker run \
    -d \
    --name smarttransit-backend \
    -p 8080:8080 \
    --env-file .env \
    --memory="512m" \
    --cpus="1" \
    --restart=unless-stopped \
    --log-driver json-file \
    --log-opt max-size=10m \
    --log-opt max-file=3 \
    smarttransit:1.0.0
```

**Docker Compose:**
```yaml
# docker-compose.yml
version: '3.8'

services:
  backend:
    image: smarttransit:1.0.0
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=smarttransit
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=smarttransit
      - JWT_SECRET=${JWT_SECRET}
    depends_on:
      - postgres
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=smarttransit
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=smarttransit
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  postgres_data:
```

**Deploy with Docker Compose:**
```bash
# Start services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f backend

# Stop services
docker-compose down
```

## Linux Server Deployment

### Using Systemd

**Create Service File:**
```bash
sudo nano /etc/systemd/system/smarttransit.service
```

**Service Configuration:**
```ini
[Unit]
Description=SmartTransit Backend Service
After=network.target

[Service]
Type=simple
User=smarttransit
WorkingDirectory=/opt/smarttransit
ExecStart=/opt/smarttransit/backend
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

# Environment
EnvironmentFile=/opt/smarttransit/.env

[Install]
WantedBy=multi-user.target
```

**Enable and Start:**
```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable smarttransit.service

# Start service
sudo systemctl start smarttransit.service

# Check status
sudo systemctl status smarttransit.service

# View logs
sudo journalctl -u smarttransit.service -f
```

**Stop Service:**
```bash
sudo systemctl stop smarttransit.service
```

## Cloud Deployment

### AWS Elastic Beanstalk

**Create .ebextensions/backend.config:**
```yaml
version: 0.2

option_settings:
  aws:elasticbeanstalk:application:environment:
    GOVERSION: 1.24.5
    DB_HOST: your-rds-endpoint.rds.amazonaws.com
    DB_PORT: 5432

commands:
  01_install:
    command: "go get -v -d ./..."
  02_build:
    command: "go build -o backend cmd/server/main.go"
```

**Deploy:**
```bash
# Initialize EB
eb init -p go smarttransit-backend

# Create environment
eb create smarttransit-prod

# Deploy
eb deploy

# Check status
eb status
```

### Google Cloud Run

**Create cloudbuild.yaml:**
```yaml
steps:
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/smarttransit:$SHORT_SHA', '.']
  
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/smarttransit:$SHORT_SHA']
  
  - name: 'gcr.io/cloud-builders/gke-deploy'
    args:
      - run
      - --filename=k8s/
      - --image=gcr.io/$PROJECT_ID/smarttransit:$SHORT_SHA
      - --location=us-central1
      - --cluster=smarttransit-cluster

images:
  - 'gcr.io/$PROJECT_ID/smarttransit:$SHORT_SHA'
```

**Deploy:**
```bash
# Submit to Cloud Build
gcloud builds submit

# Or deploy to Cloud Run directly
gcloud run deploy smarttransit \
    --image gcr.io/$PROJECT_ID/smarttransit \
    --platform managed \
    --region us-central1
```

### Kubernetes Deployment

**Create deployment.yaml:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: smarttransit-backend
spec:
  replicas: 3
  selector:
    matchLabels:
      app: smarttransit-backend
  template:
    metadata:
      labels:
        app: smarttransit-backend
    spec:
      containers:
      - name: backend
        image: smarttransit:1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          valueFrom:
            configMapKeyRef:
              name: smarttransit-config
              key: db_host
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: smarttransit-secrets
              key: jwt_secret
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: smarttransit-backend
spec:
  selector:
    app: smarttransit-backend
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

**Deploy to K8s:**
```bash
# Apply deployment
kubectl apply -f deployment.yaml

# Check status
kubectl get deployments
kubectl get pods

# View logs
kubectl logs -f deployment/smarttransit-backend

# Scale deployment
kubectl scale deployment smarttransit-backend --replicas=5
```

## Database Migration

### Pre-Deployment

1. **Backup Production Database**
```bash
pg_dump -h prod-db.example.com -U postgres smarttransit > backup_$(date +%Y%m%d).sql
```

2. **Run Migrations on New Schema**
```bash
# Test migrations in staging first
psql -h staging-db.example.com -d smarttransit -f scripts/new_migration.sql
```

### Post-Deployment

1. **Verify Data**
```bash
psql -h prod-db.example.com -d smarttransit <<EOF
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public' ORDER BY table_name;
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM bookings;
EOF
```

## Health Checks & Monitoring

### Health Endpoints

```bash
# Basic health check
curl http://localhost:8080/health

# Readiness check (ready to serve traffic)
curl http://localhost:8080/ready

# Response format:
# {
#   "status": "healthy",
#   "timestamp": "2026-02-26T10:30:00Z",
#   "version": "1.0.0"
# }
```

### Monitoring Setup

**Prometheus Metrics (if implemented):**
```bash
# Metrics endpoint
curl http://localhost:8080/metrics

# Include metrics:
# - http_requests_total
# - http_request_duration_seconds
# - db_connection_pool_usage
```

**Log Aggregation:**
- Stream logs to ELK Stack
- Configure CloudWatch (AWS)
- Or use Stackdriver (GCP)

## Rollback Procedure

### If Issues Detected

1. **Stop New Deployment**
```bash
# Docker Compose
docker-compose down

# Systemd
sudo systemctl stop smarttransit.service

# Kubernetes
kubectl rollout undo deployment/smarttransit-backend
```

2. **Restore Database** (if schema changed)
```bash
psql -h prod-db.example.com -U postgres smarttransit < backup_20260226.sql
```

3. **Start Previous Version**
```bash
# Docker
docker run -d --env-file .env smarttransit:previous-version

# Systemd
sudo systemctl start smarttransit.service

# Kubernetes
kubectl rollout undo deployment/smarttransit-backend --to-revision=1
```

## Production Environment Variables

**Never commit these to git!** Use environment variable management:

```bash
# Example for AWS Secrets Manager
aws secretsmanager create-secret \
    --name smarttransit/prod \
    --secret-string '{
        "DB_HOST": "prod-db.example.com",
        "DB_PASSWORD": "secure_password",
        "JWT_SECRET": "secure_jwt_key"
    }'
```

**Required Variables:**
```env
# Server
SERVER_PORT=8080
SERVER_LOG_LEVEL=info
GIN_MODE=release

# Database
DB_HOST=prod-db.example.com
DB_PORT=5432
DB_USER=smarttransit_user
DB_PASSWORD=SECURE_PASSWORD
DB_NAME=smarttransit

# JWT
JWT_SECRET=SECURE_JWT_SECRET_KEY
JWT_EXPIRATION_HOURS=24

# SMS Service
SMS_API_KEY=your_sms_provider_key
SMS_SENDER_ID=SmartTransit

# Email (if applicable)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=noreply@example.com
SMTP_PASSWORD=email_password
```

## Performance Tuning

### Database Connection Pool

```go
// In config/config.go
sqlDB.SetMaxOpenConns(25)      // Max connections
sqlDB.SetMaxIdleConns(5)       // Idle connections
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

### Load Balancer Configuration

**NGINX Reverse Proxy:**
```nginx
upstream smarttransit_backend {
    server backend1:8080;
    server backend2:8080;
    server backend3:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name api.smarttransit.com;

    location / {
        proxy_pass http://smarttransit_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Security Checklist

- [ ] HTTPS/TLS enabled
- [ ] No sensitive data in logs
- [ ] Database passwords in secrets manager
- [ ] JWT secrets rotated regularly
- [ ] CORS properly configured
- [ ] Rate limiting enabled
- [ ] API authentication enforced
- [ ] Audit logging implemented
- [ ] Backups automated and tested
- [ ] Firewall rules configured

## Monitoring & Alerting

### Critical Alerts

1. **Service Down**
   - Health check fails for 2+ minutes
   - Action: Auto-restart and page on-call

2. **High Error Rate**
   - Error rate > 5% for 5 minutes
   - Action: Page on-call, check logs

3. **Database Connection Issues**
   - Connection pool exhausted
   - Action: Page on-call, scale database

4. **High Latency**
   - P95 latency > 1 second
   - Action: Investigate slow queries

---

For quick reference, see [README.md](../README.md).

Questions? Check development guide or contact DevOps team.
