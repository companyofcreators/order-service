# Order Service

Core business logic service for managing orders, categories, reviews, and complaints.

## Features

- Order lifecycle management (create, assign, negotiate, complete, cancel)
- Order status history audit trail
- Tree-structured category management
- Bidirectional reviews (customer master)
- Complaint system with moderator workflow
- Kafka event publishing for order state changes

## Configuration

Copy `.env.example` to `.env` and adjust values.

## Running

```bash
go run ./cmd/api
```

## API Endpoints

All endpoints are under `/internal/` path prefix.

### Health
| Method | Path | Description |
|--------|------|-------------|
| GET | /internal/health | Health check |

### Orders
| Method | Path | Description |
|--------|------|-------------|
| POST | /internal/orders | Create order |
| GET | /internal/orders | List orders |
| GET | /internal/orders/{id} | Get order by ID |
| PATCH | /internal/orders/{id}/status | Update order status |
| POST | /internal/orders/{id}/cancel | Cancel order |
| POST | /internal/orders/{id}/complete | Complete order |
| GET | /internal/orders/{id}/history | Get status history |

### Categories
| Method | Path | Description |
|--------|------|-------------|
| GET | /internal/categories | List categories |
| POST | /internal/categories | Create category (admin) |
| PATCH | /internal/categories/{id} | Update category (admin) |
| DELETE | /internal/categories/{id} | Delete category (admin) |

### Reviews
| Method | Path | Description |
|--------|------|-------------|
| POST | /internal/reviews | Create review |
| GET | /internal/reviews/user/{id} | List reviews for user |

### Complaints
| Method | Path | Description |
|--------|------|-------------|
| POST | /internal/complaints | Create complaint |
| GET | /internal/complaints | List complaints (moderator/admin) |
| PATCH | /internal/complaints/{id} | Update complaint (moderator/admin) |

## Authorization

Read from HTTP headers:
- `X-User-Id` - user UUID
- `X-User-Role` - user role (customer, master, moderator, admin)

## Kafka Topics (Producer)

| Topic | Description |
|-------|-------------|
| order.created | Order creation event |
| order.assigned | Master assigned to order |
| order.completed | Order completion event |
| order.cancelled | Order cancellation event |
| review.created | New review event |
| complaint.created | New complaint event |

## Database Migrations

```bash
# Apply migrations
migrate -path migrations -database "$DATABASE_URL" up

# Rollback
migrate -path migrations -database "$DATABASE_URL" down
```
