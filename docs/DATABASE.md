# Database Architecture & Setup

## Overview

The SmartTransit backend uses **PostgreSQL** as its primary data store. This document covers schema design, setup instructions, and database management.

## Database Connection Details

**Driver**: pgx/v5 (async) + sqlx (synchronous wrapper)  
**Host**: localhost (configurable via `DB_HOST`)  
**Port**: 5432 (configurable via `DB_PORT`)  
**User**: postgres (configurable via `DB_USER`)  

## Schema Overview

### Core Entities

#### Users & Authentication
- `users` - Base user accounts with SMS-based authentication
- `user_sessions` - Active user sessions with JWT tokens
- `sessions` - Legacy session tracking
- `refresh_tokens` - Token refresh mechanism
- `admin_refresh_tokens` - Admin session tokens
- `admin_users` - Admin user accounts

#### Passengers
- `passengers` - Passenger profiles with contact info
- `user_sessions` - Session management

#### Bus Management
- `buses` - Bus inventory with configurations
- `bus_owners` - Bus owner organizations
- `bus_routes` - Routes operated by buses
- `bus_seat_layouts` - Seat configuration for each bus
- `bus_staff` - Bus crew members

#### Trip Management
- `master_routes` - Base route definitions
- `scheduled_trips` - Trip schedules and configurations
- `active_trips` - Currently running trips with real-time data
- `trip_seats` - Individual seat status per trip
- `route_permits` - Route operation permits

#### Booking System
- `bookings` - Passenger bookings (main entity)
- `app_bookings` - Bookings from mobile app
- `manual_bookings` - Manual bookings by staff
- `booking_intents` - Booking pipeline states
- `trip_schedules` - Trip timing and availability

#### Lounge Services
- `lounges` - Lounge facilities
- `lounge_owners` - Lounge ownership/management
- `lounge_routes` - Routes served by lounges
- `lounge_bookings` - Lounge seat reservations
- `lounge_staff` - Lounge employees

#### Financial & Audit
- `payments` - Payment records
- `payment_audits` - Payment transaction logs
- `system_settings` - Configuration values

## Setting Up the Database

### Quick Setup with Script

**Windows (PowerShell):**
```powershell
cd scripts
.\init_database.ps1
```

**Linux/Mac:**
```bash
cd scripts
chmod +x init_database.ps1
bash init_database.ps1
```

### Manual Setup

1. **Create Database**
```sql
CREATE DATABASE smarttransit;
```

2. **Connect to Database**
```bash
psql -h localhost -U postgres -d smarttransit
```

3. **Run Schema Initialization**
```bash
psql -h localhost -U postgres -d smarttransit -f scripts/01_init_database_schema.sql
```

4. **Verify Tables**
```sql
\dt  -- List all tables
\d users  -- Describe users table
```

## Key Relationships

```
users (1) ── (M) user_sessions
users (1) ── (M) refresh_tokens
passengers (1) ── (M) bookings
buses (1) ── (M) scheduled_trips
scheduled_trips (1) ── (M) trips_seats
scheduled_trips (1) ── (M) bookings
lounges (1) ── (M) lounge_bookings
lounge_bookings (M) ── (1) lounges
bus_routes (1) ── (M) buses
```

## Important Constraints

- **Foreign Keys**: All relationships have foreign key constraints
- **Unique Constraints**: Phone numbers, email addresses must be unique per user type
- **Check Constraints**: Seat numbers, prices must be positive
- **NOT NULL**: Critical fields like user_id, booking_id, etc.

## Data Access Layer

The backend uses the **Repository Pattern** for database access:

```
Resource → Handler → Service → Repository → Database
```

### Repository Files
Each entity has a dedicated repository file in `internal/database/`:
- `user_repository.go` - User CRUD operations
- `booking_repository.go` - Booking management
- `bus_repository.go` - Bus operations
- `lounge_repository.go` - Lounge operations
- etc.

### Sample Repository Interface
```go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    GetByPhone(ctx context.Context, phone string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter Filter) ([]*User, error)
}
```

## Common Database Operations

### Insert/Create
```go
repo := database.NewUserRepository(db)
user := &User{
    ID: uuid.New().String(),
    Phone: "+94712345678",
    Name: "John Doe",
}
err := repo.Create(ctx, user)
```

### Query/Read
```go
user, err := repo.GetByID(ctx, userID)
users, err := repo.List(ctx, Filter{Role: "passenger"})
```

### Update
```go
user.Email = "newemail@example.com"
err := repo.Update(ctx, user)
```

### Delete
```go
err := repo.Delete(ctx, userID)
```

## Migration Scripts

Located in `scripts/` folder for database maintenance:

| Script | Purpose |
|--------|---------|
| `01_init_database_schema.sql` | Initial schema creation |
| `fix_*.sql` | Bug fixes for specific issues |
| `diagnose_*.sql` | Diagnostic queries |
| `clear_all_data.sql` | Reset database (⚠️ destructive) |

## Performance Considerations

### Indexes
Key indexes are created on:
- Primary keys (automatic)
- Foreign keys (for joins)
- `users.phone` (for login)
- `bookings.user_id` (for queries)
- `trips.scheduled_trip_id` (for searches)

### Query Optimization
- Use connection pooling (configured in code)
- Batch writes when possible
- Use transactions for multi-entity operations

## Backing Up & Restoring

### Backup
```bash
pg_dump -h localhost -U postgres smarttransit > backup.sql
```

### Restore
```bash
psql -h localhost -U postgres smarttransit < backup.sql
```

## Troubleshooting

### Connection Issues
```bash
# Check PostgreSQL is running
sudo systemctl status postgresql  # Linux
pg_isready -h localhost  # Test connection
```

### Missing Tables
```sql
-- Check existing tables
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public';
```

### Data Integrity Issues
```sql
-- Find orphaned records
SELECT * FROM bookings b 
WHERE NOT EXISTS (SELECT 1 FROM passengers p WHERE p.id = b.passenger_id);
```

## Best Practices

1. **Always use transactions** for multi-entity operations
2. **Validate data** before inserting (handled by services)
3. **Use prepared statements** (handled by sqlx/pgx)
4. **Regular backups** of production database
5. **Monitor query performance** with `EXPLAIN ANALYZE`
6. **Log database errors** for debugging

---

For implementation details, see the repository files in `internal/database/`.
