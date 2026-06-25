# Development Guide

This guide covers everything needed to develop and test the SmartTransit backend.

## Prerequisites

### Required Software
- **Go**: 1.24.5 or higher
  - [Download Go](https://go.dev/dl/)
  - Verify: `go version`

- **PostgreSQL**: 12 or higher
  - [Download PostgreSQL](https://www.postgresql.org/download/)
  - Verify: `psql --version`

- **Git**: For version control
  - Verify: `git --version`

### Optional Tools
- **Make**: For running build commands
  - Windows: Install via [Chocolatey](https://chocolatey.org): `choco install make`
  - Linux/Mac: Usually pre-installed

- **air**: For hot reload during development
  ```bash
  go install github.com/cosmtrek/air@latest
  ```

- **sqlc**: For SQL code generation (optional)
  ```bash
  go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
  ```

- **golangci-lint**: For linting
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```

## Project Setup

### 1. Clone Repository
```bash
git clone <repository-url>
cd backend
```

### 2. Install Dependencies
```bash
go mod download
go mod tidy
```

### 3. Configure Environment
```bash
# Copy example configuration
cp .env.example .env

# Edit .env with your settings
# Windows
notepad .env
# Linux/Mac
nano .env
```

### 4. Setup Database
```bash
cd scripts

# Windows (PowerShell)
.\init_database.ps1

# Linux/Mac
chmod +x init_database.ps1
./init_database.ps1
```

### 5. Verify Setup
```bash
# Test database connection
go run cmd/server/main.go

# Should see: "Starting SmartTransit SMS Authentication Backend"
# Stop with Ctrl+C
```

## Development Workflow

### Running the Server

**Basic Run:**
```bash
go run cmd/server/main.go
```

**With Hot Reload (auto-restart on file changes):**
```bash
air
```

**Background Run (keep terminal free):**
```bash
# Windows
Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "cmd/server/main.go"

# Linux/Mac
go run cmd/server/main.go &
```

### Code Structure

When adding new features, follow this pattern:

#### 1. Define Model (`internal/models/`)
```go
type User struct {
    ID        string    `db:"id"`
    Name      string    `db:"name"`
    Phone     string    `db:"phone"`
    CreatedAt time.Time `db:"created_at"`
}
```

#### 2. Create Repository (`internal/database/`)
```go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    List(ctx context.Context) ([]*User, error)
}

type userRepository struct {
    db *sql.DB
}

func (r *userRepository) Create(ctx context.Context, user *User) error {
    query := `INSERT INTO users (id, name, phone, created_at) 
             VALUES ($1, $2, $3, $4)`
    _, err := r.db.ExecContext(ctx, query, user.ID, user.Name, user.Phone, user.CreatedAt)
    return err
}
```

#### 3. Create Service (`internal/services/`)
```go
type UserService interface {
    RegisterUser(ctx context.Context, phone string) error
    GetUser(ctx context.Context, id string) (*User, error)
}

type userService struct {
    userRepo UserRepository
    smsRepo  SMSRepository
}

func (s *userService) RegisterUser(ctx context.Context, phone string) error {
    // Business logic
    user := &User{ID: uuid.New().String(), Phone: phone}
    return s.userRepo.Create(ctx, user)
}
```

#### 4. Create Handler (`internal/handlers/`)
```go
type UserHandler struct {
    userService UserService
}

func (h *UserHandler) Register(c *gin.Context) {
    var req RegisterRequest
    
    // Parse request
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Call service
    err := h.userService.RegisterUser(c.Request.Context(), req.Phone)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // Return response
    c.JSON(200, gin.H{"message": "registered"})
}
```

#### 5. Register Routes (`cmd/server/main.go`)
```go
userHandler := handlers.NewUserHandler(userService)
router.POST("/api/auth/register", userHandler.Register)
router.GET("/api/users/:id", userHandler.GetUser)
```

## Testing

### Unit Tests

**Structure:**
```go
// file: internal/services/user_service_test.go
func TestRegisterUser(t *testing.T) {
    // Arrange
    mockRepo := &MockUserRepository{}
    service := NewUserService(mockRepo)
    
    // Act
    err := service.RegisterUser(context.Background(), "+94712345678")
    
    // Assert
    assert.NoError(t, err)
    assert.True(t, mockRepo.CreateCalled)
}
```

**Run Tests:**
```bash
# All tests
go test ./...

# Specific package
go test ./internal/services

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

### Integration Tests

Testing with real database:
```go
func TestUserRepositoryIntegration(t *testing.T) {
    // Setup test database
    db := setupTestDB()
    defer db.Close()
    
    repo := NewUserRepository(db)
    
    // Test CRUD operations
    user := &User{ID: "test-1", Name: "Test", Phone: "+94712345678"}
    err := repo.Create(context.Background(), user)
    require.NoError(t, err)
    
    retrieved, err := repo.GetByID(context.Background(), "test-1")
    require.NoError(t, err)
    assert.Equal(t, user.Name, retrieved.Name)
}
```

### API Testing

**Manual Testing with curl:**
```bash
# Register user
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"phone":"+94712345678","name":"John"}'

# Search trips
curl "http://localhost:8080/api/trips/search?from=CMB&to=KDY&date=2026-03-01"

# With authentication
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/bookings
```

**Postman/Insomnia:**
1. Import `swagger.yaml` into Postman
2. Use the generated collection to test endpoints
3. Configure environment variables for token, IDs, etc.

**Shell Script Testing:**
```bash
# Use provided test script
bash test_search_api.sh
```

## Database Management

### Migrations

**Create New Migration:**
1. Add SQL file to `scripts/` folder
2. Follow naming: `001_create_users.sql`
3. Run: `psql -d smarttransit -f scripts/001_create_users.sql`

**Rollback (if applicable):**
- Restore from backup
- Or manually DROP tables in reverse order

### Schema Changes

**Safe Schema Changes:**
1. Always backup first: `pg_dump smarttransit > backup.sql`
2. Make change in test environment
3. Test thoroughly
4. Apply to production using downtime window

### Data Inspection

**Connect to Database:**
```bash
psql -h localhost -U postgres -d smarttransit
```

**Common Queries:**
```sql
-- List tables
\dt

-- Describe table
\d users

-- Count records
SELECT COUNT(*) FROM users;

-- View sample data
SELECT * FROM users LIMIT 10;

-- Find specific user
SELECT * FROM users WHERE phone = '+94712345678';
```

## Code Quality

### Linting

```bash
# Run linter
golangci-lint run

# Fix auto-fixable issues
golangci-lint run --fix
```

### Code Formatting

```bash
# Format single file
gofmt -w internal/handlers/user_handler.go

# Format entire project
go fmt ./...
```

### Best Practices

1. **Error Handling**
   ```go
   // Good
   if err != nil {
       logrus.WithError(err).Error("failed to create user")
       return fmt.Errorf("failed to create user: %w", err)
   }
   
   // Bad
   if err != nil {
       panic(err)  // Don't panic in production code
   }
   ```

2. **Logging**
   ```go
   // Use structured logging
   logrus.WithFields(logrus.Fields{
       "user_id": userID,
       "action": "login",
   }).Info("User logged in")
   ```

3. **Context Usage**
   ```go
   // Always accept and use context
   func (s *Service) DoWork(ctx context.Context) error {
       select {
       case <-ctx.Done():
           return ctx.Err()
       // ... work ...
       }
   }
   ```

4. **Validation**
   ```go
   // Use validator package
   if err := validator.Validate(user); err != nil {
       return fmt.Errorf("invalid user: %w", err)
   }
   ```

5. **Constants**
   ```go
   // Define magic strings as constants
   const (
       RolePassenger = "passenger"
       RoleBusOwner = "bus_owner"
       RoleAdmin = "admin"
   )
   ```

## Debugging

### Debug Logs

Set log level to DEBUG:
```bash
SERVER_LOG_LEVEL=debug go run cmd/server/main.go
```

### Breakpoint Debugging (Delve)

```bash
# Install Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug cmd/server/main.go

# In debugger shell:
# break main.main  - Set breakpoint
# continue         - Run until breakpoint
# next             - Step next line
# print variable   - Print variable value
# quit             - Exit
```

### Request Tracing

Use logging middleware to trace requests:
```go
logrus.WithFields(logrus.Fields{
    "method": c.Request.Method,
    "path": c.Request.URL.Path,
    "ip": c.ClientIP(),
}).Debug("Incoming request")
```

## Environment Configuration

### Development (.env)
```env
SERVER_PORT=8080
SERVER_LOG_LEVEL=debug
GIN_MODE=debug

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=smarttransit_dev

JWT_SECRET=dev_secret_key
JWT_EXPIRATION_HOURS=24
```

### Testing (.env.test)
```env
SERVER_PORT=8081
SERVER_LOG_LEVEL=error
GIN_MODE=test

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=smarttransit_test
```

### Production (.env.prod)
```env
SERVER_PORT=8080
SERVER_LOG_LEVEL=info
GIN_MODE=release

DB_HOST=prod-db.example.com
DB_PORT=5432
DB_USER=prod_user
DB_PASSWORD=SECURE_PASSWORD
DB_NAME=smarttransit_prod

JWT_SECRET=SECURE_JWT_SECRET
JWT_EXPIRATION_HOURS=24
```

## Building

### Build Binary

```bash
# Standard build
go build -o backend cmd/server/main.go

# With version info
go build -ldflags="-X main.version=1.0.0" -o backend cmd/server/main.go

# Run the binary
./backend
```

### Docker Build

```bash
# Build image
docker build -t smarttransit:latest .

# Run container
docker run -p 8080:8080 --env-file .env smarttransit:latest
```

## Version Control

### Git Workflow

```bash
# Create feature branch
git checkout -b feature/user-registration

# Make changes
git add .
git commit -m "feat: Add user registration endpoint"

# Before pushing, ensure tests pass
go test ./...

# Push to remote
git push origin feature/user-registration

# Create Pull Request on GitHub
```

### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

**Types:** feat, fix, docs, style, refactor, test, chore

**Example:**
```
feat: Add SMS OTP verification

Implement OTP verification endpoint for user authentication
using SMS service integration.

Closes #123
```

## Troubleshooting Development Issues

### Port Already in Use
```bash
# Find and kill process using port 8080
# Windows
netstat -ano | findstr :8080
taskkill /PID <PID> /F

# Linux/Mac
lsof -i :8080
kill -9 <PID>
```

### Database Connection Issues
```bash
# Test connection
psql -h localhost -U postgres -d smarttransit -c "SELECT 1"

# Check PostgreSQL service
# Windows: Services app or `Get-Service postgresql*`
# Linux: `sudo systemctl status postgresql`
```

### Module Import Issues
```bash
# Clear module cache
go clean -modcache

# Verify module
go mod verify

# Download dependencies
go mod download

# Clean up unused
go mod tidy
```

## IDE Setup

### VS Code

**Extensions:**
```json
{
    "recommendations": [
        "golang.go",
        "eamodio.gitlens",
        "ms-vscode.makefile-tools",
        "ms-mssql.mssql"
    ]
}
```

**settings.json:**
```json
{
    "[go]": {
        "editor.formatOnSave": true,
        "editor.codeActionsOnSave": {
            "source.fixAll": true
        }
    },
    "go.lintOnSave": "package",
    "go.lintTool": "golangci-lint"
}
```

### GoLand/IntelliJ IDEA
- Enable Go plugin
- Configure Go SDK path
- Enable gofmt on save
- Configure golangci-lint

---

Happy coding! For questions, check code comments or documentation files.
