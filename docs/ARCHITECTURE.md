# System Architecture

## High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     Client Applications                          │
│            (Mobile App, Web, Admin Dashboard)                    │
└──────────────────────────────┬──────────────────────────────────┘
                               │ HTTP/HTTPS
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      API Gateway Layer                           │
│  • CORS Handling  • Request Logging  • Rate Limiting             │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Middleware Stack                              │
│  • JWT Authentication  • Authorization  • Request Validation     │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Handler Layer                               │
│  • Route Handlers  • Request Processing  • Response Formatting   │
│                                                                  │
│  ├─ AuthHandler          ├─ BookingHandler  ├─ SearchHandler   │
│  ├─ BusOwnerHandler      ├─ LoungeHandler   ├─ AdminHandler    │
│  └─ Other Handlers...                                            │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Service Layer                               │
│  • Business Logic  • Orchestration  • Validation                 │
│                                                                  │
│  ├─ AuthService          ├─ BookingService  ├─ SearchService   │
│  ├─ BusService           ├─ LoungeService   ├─ PaymentService  │
│  └─ Other Services...                                            │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Repository Layer (DAL)                         │
│  • Database Queries  • Entity Mapping  • Transaction Management   │
│                                                                  │
│  ├─ UserRepository       ├─ BookingRepository                   │
│  ├─ BusRepository        ├─ TripRepository                      │
│  └─ Other Repositories...                                        │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Database Layer                            │
│                   PostgreSQL 12+                                │
│                   (Relational Database)                         │
└─────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│                    External Services                              │
│                                                                  │
│  ├─ SMS Gateway (OTP)        ├─ Payment Provider               │
│  ├─ Email Service            ├─ Location Services              │
│  └─ Notification Services                                        │
└──────────────────────────────────────────────────────────────────┘
```

## Layered Architecture Pattern

The SmartTransit backend follows a classic **3-tier layered architecture**:

### 1. Presentation Layer (Handlers)
**Location**: `internal/handlers/`

Responsible for:
- Receiving HTTP requests
- Parsing and validating input
- Calling appropriate services
- Formatting responses
- Handling HTTP status codes

Example Handler Structure:
```go
type UserHandler struct {
    userService UserService
}

func (h *UserHandler) GetUser(c *gin.Context) {
    userID := c.Param("id")
    
    // Validation
    if userID == "" {
        c.JSON(400, gin.H{"error": "invalid user id"})
        return
    }
    
    // Call service
    user, err := h.userService.GetUser(ctx, userID)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // Return response
    c.JSON(200, user)
}
```

### 2. Business Logic Layer (Services)
**Location**: `internal/services/`

Responsible for:
- Complex business logic
- Data validation
- Orchestrating multiple repositories
- Transaction management
- Domain-specific rules

Example Service Structure:
```go
type BookingService struct {
    bookingRepo BookingRepository
    tripRepo    TripRepository
    paymentSvc  PaymentService
}

func (s *BookingService) CreateBooking(ctx context.Context, req CreateBookingRequest) error {
    // Validate business rules
    if err := s.validateBooking(req); err != nil {
        return err
    }
    
    // Call multiple repositories
    trip, err := s.tripRepo.GetByID(ctx, req.TripID)
    if err != nil {
        return err
    }
    
    // Process payment
    payment, err := s.paymentSvc.Process(ctx, req.Amount)
    if err != nil {
        return err
    }
    
    // Save booking
    booking := &Booking{...}
    return s.bookingRepo.Create(ctx, booking)
}
```

### 3. Data Access Layer (Repositories)
**Location**: `internal/database/`

Responsible for:
- Database queries
- CRUD operations
- Entity mapping
- Connection management
- Query optimization

Example Repository Structure:
```go
type UserRepository struct {
    db *sql.DB
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
    query := `SELECT id, name, phone, email FROM users WHERE id = $1`
    
    var user User
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &user.ID, &user.Name, &user.Phone, &user.Email,
    )
    
    return &user, err
}
```

## Request Flow Example

### User Registration Flow

```
1. HTTP Request (POST /api/auth/register)
   └─> Input: {phone: "+94712345678"}

2. AuthHandler.Register()
   └─> Validates phone format
   └─> Calls AuthService.Register()

3. AuthService.Register()
   └─> Checks if user exists (UserRepository.GetByPhone)
   └─> Generates OTP
   └─> Saves user + OTP (UserRepository.Create)
   └─> Sends SMS (SMSService.SendOTP)
   └─> Returns OTP reference

4. UserRepository.Create()
   └─> Executes INSERT query in PostgreSQL
   └─> Returns user ID

5. SMSService.SendOTP()
   └─> Calls external SMS API
   └─> Returns confirmation

6. Handler returns HTTP 200
   └─> Response: {status: "otp_sent", otpReference: "xxx"}

7. Client receives response
```

## Key Design Patterns

### 1. Repository Pattern
Abstracts database access:
```go
type Repository interface {
    Create(ctx context.Context, entity Entity) error
    GetByID(ctx context.Context, id string) (Entity, error)
    Update(ctx context.Context, entity Entity) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter Filter) ([]Entity, error)
}
```

### 2. Dependency Injection
Services receive dependencies via constructors:
```go
userService := NewUserService(userRepository, smsService)
handler := NewUserHandler(userService)
```

### 3. Error Handling
Consistent error responses:
```go
if err != nil {
    logrus.WithError(err).Error("operation failed")
    return fmt.Errorf("failed to create user: %w", err)
}
```

### 4. Context Propagation
Context flows through all layers for timeouts and cancellation:
```go
func (h *Handler) ProcessRequest(c *gin.Context) {
    ctx := c.Request.Context()
    result, err := h.service.DoWork(ctx)  // Context passed down
}
```

### 5. Middleware Pipeline
Request processing chain:
```
Request
  ├─> CORS Middleware
  ├─> Logging Middleware
  ├─> Auth Middleware
  ├─> Route Handler
  └─> Response
```

## Concurrency & Performance

### Connection Pooling
PostgreSQL connections are pooled:
- Reduces connection overhead
- Reuses connections
- Configurable pool size

### Async Operations
Go's goroutines enable:
- Concurrent request handling
- Parallel database queries
- Non-blocking I/O

### Graceful Shutdown
Server stops accepting new requests, completes inflight operations:
```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
<-sigChan
server.Shutdown(context.Background())
```

## Security Architecture

### Authentication Flow
```
Client Request
    ↓
Extract JWT Token from Header
    ↓
Validate Signature (JWT secret)
    ↓
Check Expiration
    ↓
Extract Claims (userID, role)
    ↓
Store in Context
    ↓
Handler accesses ctx.GetString("userID")
```

### Authorization
Role-based access control:
- `passenger` - Access own bookings
- `bus_owner` - Manage buses and routes
- `lounge_owner` - Manage lounges
- `admin` - Full system access
- `staff` - Limited operational access

### Password Security
- SMS-based OTP (no stored passwords initially)
- Future: Bcrypt hashing for passwords
- JWT tokens for session management
- Refresh token rotation support

## Scalability Considerations

### Current Architecture
- Single Go process
- Shared database connection pool
- In-memory middleware state

### Future Improvements
1. **Horizontal Scaling**
   - Load balancer in front
   - Shared session store (Redis)
   - Distributed logging (ELK stack)

2. **Database Scaling**
   - Read replicas for searches
   - Write primary for mutations
   - Connection pooling service (PgBouncer)

3. **Caching**
   - Redis for session data
   - Cache frequently accessed routes/trips
   - Cache invalidation on updates

4. **Message Queue**
   - Separate SMS/email sending to queue
   - Asynchronous notifications
   - Retry mechanism for failed sends

## Code Organization Best Practices

1. **Cohesion**: Related code stays together
2. **Coupling**: Minimize dependencies between packages
3. **Package Structure**: One main responsibility per package
4. **Naming**: Clear, descriptive names (UserHandler, not UH)
5. **Comments**: Explain "why", not "what"
6. **Testing**: Each layer testable independently

## Monitoring & Logging

### Logging Strategy
- **INFO**: Application lifecycle events
- **WARN**: Recoverable issues
- **ERROR**: Failed operations
- **DEBUG**: Detailed execution flow

### Key Metrics
- Request count and latency
- Database query performance
- Error rates by endpoint
- Active connections
- Payment transaction status

---

For implementation details, explore the source code and service documentation.
