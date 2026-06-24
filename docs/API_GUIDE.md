# API Reference Guide

Complete reference for all SmartTransit backend API endpoints.

## Base URL

```
http://localhost:8080/api
```

or for production:
```
https://api.smarttransit.com/api
```

## Authentication

Most endpoints require a JWT token in the Authorization header:

```bash
Authorization: Bearer <jwt_token>
```

### Getting a Token

1. Register or login to get a JWT token
2. Include token in subsequent requests
3. Tokens expire after 24 hours
4. Use refresh endpoint to get new token

## Response Format

All responses follow this format:

**Success (2xx):**
```json
{
  "data": { /* Response payload */ },
  "status": "success",
  "timestamp": "2026-02-26T10:30:00Z"
}
```

**Error (4xx/5xx):**
```json
{
  "error": "Error message",
  "status": "error",
  "code": "ERROR_CODE",
  "timestamp": "2026-02-26T10:30:00Z"
}
```

## Authentication Endpoints

### Register User
Create a new user account with phone number.

**Endpoint:** `POST /auth/register`

**No Authentication Required**

**Request Body:**
```json
{
  "phone": "+94712345678",
  "full_name": "John Doe",
  "email": "john@example.com",
  "user_type": "passenger"
}
```

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| phone | string | Yes | E.164 format: +<country_code><number> |
| full_name | string | Yes | User's full name |
| email | string | No | User email address |
| user_type | string | Yes | One of: `passenger`, `bus_owner`, `lounge_owner` |

**Response (200):**
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "otp_reference": "OTP_REF_123",
    "expires_in": 300,
    "message": "OTP sent to registered phone"
  }
}
```

**Errors:**
- `400` - Invalid phone format
- `409` - Phone already registered
- `500` - Server error

---

### Verify OTP
Verify OTP sent to phone and get JWT token.

**Endpoint:** `POST /auth/verify-otp`

**No Authentication Required**

**Request Body:**
```json
{
  "phone": "+94712345678",
  "otp": "123456",
  "otp_reference": "OTP_REF_123"
}
```

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| phone | string | Yes | User's phone number |
| otp | string | Yes | 6-digit OTP code |
| otp_reference | string | Yes | Reference from register response |

**Response (200):**
```json
{
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 86400,
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "phone": "+94712345678",
      "name": "John Doe",
      "role": "passenger"
    }
  }
}
```

**Errors:**
- `400` - Invalid OTP
- `401` - OTP expired
- `404` - Phone not found
- `500` - Server error

---

### Refresh Token
Get a new JWT token using refresh token.

**Endpoint:** `POST /auth/refresh`

**No Authentication Required**

**Request Body:**
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response (200):**
```json
{
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 86400
  }
}
```

---

### Logout
Invalidate current session.

**Endpoint:** `POST /auth/logout`

**Authentication Required**

**Response (200):**
```json
{
  "data": {
    "message": "Successfully logged out"
  }
}
```

---

## Booking Endpoints

### Create Booking
Create a new bus trip booking.

**Endpoint:** `POST /bookings`

**Authentication Required** (Bearer token)

**Request Body:**
```json
{
  "trip_id": "trip_123",
  "seat_numbers": [1, 2],
  "passenger_details": {
    "name": "John Doe",
    "email": "john@example.com",
    "phone": "+94712345678"
  },
  "payment_method": "card"
}
```

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| trip_id | string | Yes | ID of trip to book |
| seat_numbers | array | Yes | Array of seat numbers (1-indexed) |
| passenger_details | object | Yes | Passenger information |
| payment_method | string | Yes | One of: `card`, `cash`, `wallet` |

**Response (201):**
```json
{
  "data": {
    "booking_id": "BK_550e8400",
    "trip_id": "trip_123",
    "seats": [1, 2],
    "status": "pending_payment",
    "total_price": 1800.00,
    "created_at": "2026-02-26T10:30:00Z"
  }
}
```

---

### Get Booking Details
Retrieve specific booking information.

**Endpoint:** `GET /bookings/:booking_id`

**Authentication Required**

**Response (200):**
```json
{
  "data": {
    "id": "BK_550e8400",
    "user_id": "user_123",
    "trip_id": "trip_123",
    "seats": [1, 2],
    "status": "confirmed",
    "price": 1800.00,
    "payment_status": "paid",
    "created_at": "2026-02-26T10:30:00Z",
    "departure_time": "2026-03-01T14:30:00Z"
  }
}
```

---

### List User Bookings
Get all bookings for current user.

**Endpoint:** `GET /bookings`

**Authentication Required**

**Query Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| status | string | Filter by status: `pending`, `confirmed`, `cancelled` |
| from_date | date | Filter from date (YYYY-MM-DD) |
| to_date | date | Filter to date (YYYY-MM-DD) |
| limit | number | Results per page (default: 20) |
| offset | number | Pagination offset (default: 0) |

**Example:**
```
GET /bookings?status=confirmed&limit=10&offset=0
```

**Response (200):**
```json
{
  "data": [
    {
      "id": "BK_550e8400",
      "trip_id": "trip_123",
      "seats": [1, 2],
      "status": "confirmed",
      "total_price": 1800.00,
      "departure_time": "2026-03-01T14:30:00Z"
    }
  ],
  "pagination": {
    "total": 15,
    "limit": 10,
    "offset": 0,
    "has_more": true
  }
}
```

---

### Cancel Booking
Cancel an existing booking.

**Endpoint:** `DELETE /bookings/:booking_id`

**Authentication Required**

**Request Body (optional):**
```json
{
  "reason": "Change of plans"
}
```

**Response (200):**
```json
{
  "data": {
    "message": "Booking cancelled successfully",
    "refund_amount": 1800.00,
    "cancellation_fee": 0.00
  }
}
```

---

## Search Endpoints

### Search Trips
Search available trips based on route and date.

**Endpoint:** `GET /trips/search`

**No Authentication Required**

**Query Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| from | string | Yes | Departure location code or name |
| to | string | Yes | Arrival location code or name |
| date | date | Yes | Travel date (YYYY-MM-DD) |
| travelers | number | No | Number of passengers (default: 1) |

**Example:**
```
GET /trips/search?from=CMB&to=KDY&date=2026-03-01&travelers=2
```

**Response (200):**
```json
{
  "data": [
    {
      "trip_id": "trip_123",
      "bus": {
        "id": "bus_456",
        "name": "Intercity Express",
        "operator": "Bus Company XYZ",
        "type": "sleeper"
      },
      "route": {
        "from": "Colombo",
        "to": "Kandy",
        "departure_time": "2026-03-01T14:30:00Z",
        "arrival_time": "2026-03-01T17:30:00Z",
        "duration_minutes": 180
      },
      "pricing": {
        "price_per_seat": 900.00,
        "total_seats": 45,
        "available_seats": 12
      },
      "amenities": ["AC", "WiFi", "Food"]
    }
  ],
  "count": 5
}
```

---

### Get Trip Details
Get detailed information about a specific trip.

**Endpoint:** `GET /trips/:trip_id`

**No Authentication Required**

**Response (200):**
```json
{
  "data": {
    "id": "trip_123",
    "bus": {
      "id": "bus_456",
      "name": "Intercity Express",
      "license_plate": "ABC-1234",
      "seats_layout": {
        "total": 45,
        "configuration": "1-2",
        "occupied": 33,
        "available": 12
      }
    },
    "route": {
      "from": "Colombo",
      "to": "Kandy",
      "departure_time": "2026-03-01T14:30:00Z",
      "arrival_time": "2026-03-01T17:30:00Z",
      "stops": ["Dambulla"]
    },
    "pricing": {
      "base_price": 900.00,
      "taxes": 100.00,
      "discount": 0.00,
      "total": 1000.00
    },
    "available_seats": [1, 2, 5, 7, 8, 10, 12, 15, 18, 20, 25, 30],
    "operator": {
      "id": "op_789",
      "name": "Bus Company XYZ",
      "rating": 4.5
    }
  }
}
```

---

### Get Active Trips
Get currently running trips (for drivers/tracking).

**Endpoint:** `GET /trips/active`

**Authentication Required** (Admin/Staff role)

**Response (200):**
```json
{
  "data": [
    {
      "trip_id": "trip_123",
      "bus": "bus_456",
      "current_location": {
        "latitude": 6.9271,
        "longitude": 80.7789,
        "address": "Colombo, Sri Lanka"
      },
      "progress": {
        "distance_covered": 45.2,
        "total_distance": 115.0,
        "percent": 39.3
      },
      "status": "in_transit",
      "passengers_onboard": 33,
      "next_stop": "Dambulla"
    }
  ]
}
```

---

## Bus Management Endpoints (Bus Owner)

### Create Bus
Register a new bus.

**Endpoint:** `POST /buses`

**Authentication Required** (bus_owner role)

**Request Body:**
```json
{
  "name": "Intercity Express",
  "license_plate": "ABC-1234",
  "type": "sleeper",
  "capacity": 45,
  "manufacturer": "Volvo",
  "year": 2022,
  "ac": true,
  "wifi": true,
  "amenities": ["USB Charging", "Food Service"]
}
```

**Response (201):**
```json
{
  "data": {
    "id": "bus_456",
    "name": "Intercity Express",
    "license_plate": "ABC-1234",
    "status": "active"
  }
}
```

---

### List Buses
Get all buses owned by current bus owner.

**Endpoint:** `GET /buses`

**Authentication Required** (bus_owner role)

**Response (200):**
```json
{
  "data": [
    {
      "id": "bus_456",
      "name": "Intercity Express",
      "license_plate": "ABC-1234",
      "type": "sleeper",
      "capacity": 45,
      "status": "active",
      "operating_routes": 5,
      "total_bookings": 150
    }
  ]
}
```

---

### Update Bus
Modify bus details.

**Endpoint:** `PUT /buses/:bus_id`

**Authentication Required**

**Request Body:**
```json
{
  "name": "Intercity Express Pro",
  "amenities": ["AC", "WiFi", "USB Charging", "Food"]
}
```

**Response (200):**
```json
{
  "data": {
    "id": "bus_456",
    "message": "Bus updated successfully"
  }
}
```

---

## Lounge Endpoints (Lounge Owner)

### Create Lounge
Register a new lounge facility.

**Endpoint:** `POST /lounges`

**Authentication Required** (lounge_owner role)

**Request Body:**
```json
{
  "name": "Central Lounge",
  "location": "Colombo Central",
  "address": "100 Main St, Colombo",
  "capacity": 100,
  "amenities": ["WiFi", "Rest Area", "Cafeteria", "Parking"],
  "operating_hours": {
    "open": "04:00",
    "close": "22:00"
  }
}
```

**Response (201):**
```json
{
  "data": {
    "id": "lounge_789",
    "name": "Central Lounge",
    "status": "active"
  }
}
```

---

### Create Lounge Route
Add a route served by lounge.

**Endpoint:** `POST /lounge-routes`

**Authentication Required**

**Request Body:**
```json
{
  "lounge_id": "lounge_789",
  "from": "Colombo",
  "to": "Kandy",
  "price": 500.00
}
```

**Response (201):**
```json
{
  "data": {
    "id": "route_123",
    "from": "Colombo",
    "to": "Kandy",
    "price": 500.00
  }
}
```

---

### Create Lounge Booking
Book lounge space (for waiting before journey).

**Endpoint:** `POST /lounge-bookings`

**Authentication Required**

**Request Body:**
```json
{
  "lounge_id": "lounge_789",
  "lounge_route_id": "route_123",
  "date": "2026-03-01",
  "check_in_time": "12:00",
  "check_out_time": "14:30",
  "number_of_people": 2
}
```

**Response (201):**
```json
{
  "data": {
    "booking_id": "LB_550e8400",
    "lounge_id": "lounge_789",
    "status": "confirmed",
    "price": 1000.00
  }
}
```

---

## Admin Endpoints

### Get All Users
List all users in system.

**Endpoint:** `GET /admin/users`

**Authentication Required** (admin role)

**Query Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| role | string | Filter by role |
| status | string | Filter by status |
| limit | number | Results per page |
| offset | number | Pagination offset |

**Response (200):**
```json
{
  "data": [
    {
      "id": "user_123",
      "name": "John Doe",
      "phone": "+94712345678",
      "email": "john@example.com",
      "role": "passenger",
      "status": "active",
      "created_at": "2026-02-26T10:30:00Z",
      "total_bookings": 5
    }
  ],
  "pagination": {
    "total": 250,
    "limit": 20,
    "offset": 0
  }
}
```

---

### Get System Analytics
Get system-wide statistics.

**Endpoint:** `GET /admin/analytics`

**Authentication Required** (admin role)

**Response (200):**
```json
{
  "data": {
    "total_users": 5000,
    "total_bookings": 15000,
    "total_revenue": 25000000.00,
    "active_trips": 45,
    "buses_online": 120,
    "average_rating": 4.3,
    "top_routes": [
      {
        "from": "Colombo",
        "to": "Kandy",
        "bookings": 1200
      }
    ]
  }
}
```

---

## Error Codes

| Code | HTTP | Description |
|------|------|-------------|
| `INVALID_REQUEST` | 400 | Malformed request or invalid parameters |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication token |
| `FORBIDDEN` | 403 | User lacks permission for this action |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource already exists or conflict detected |
| `RATE_LIMITED` | 429 | Too many requests |
| `SERVER_ERROR` | 500 | Internal server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

---

## Rate Limiting

API requests are rate-limited:
- **Anonymous**: 100 requests/hour
- **Authenticated**: 1000 requests/hour
- **Admin**: Unlimited

Rate limit headers:
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1645862400
```

---

For Swagger/OpenAPI documentation, see [swagger.yaml](../swagger.yaml) or visit `/swagger/index.html` when running the server.
