# Backend Golang Coding Test — Part 1: User Management API

A RESTful user management API built with Go, MongoDB, and JWT authentication, structured using Hexagonal Architecture (ports and adapters).

---

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [JWT Guide](#jwt-guide)
- [Sample Requests and Responses](#sample-requests-and-responses)
- [Testing](#testing)
- [Design Decisions and Assumptions](#design-decisions-and-assumptions)
- [Requirements Checklist](#requirements-checklist)

---

## Quick Start

### Option A — Everything in Docker (recommended)

```bash
git clone <repository-url>
cd backend-challenge

docker compose up --build -d
curl http://localhost:8080/health
```

The API is available at `http://localhost:8080`. MongoDB runs alongside it and the API waits for MongoDB to pass its healthcheck before starting.

To view logs:

```bash
docker compose logs -f api
```

To stop:

```bash
docker compose down          # keeps data
docker compose down -v       # also removes the database volume
```

### Option B — MongoDB in Docker, API on the host

```bash
docker compose up -d mongo

cp .env.example .env
# edit .env and set JWT_SECRET

go mod download
go run ./cmd/api
```

### Prerequisites

| Tool | Version |
| --- | --- |
| Go | 1.22 or later |
| Docker | 20.10 or later |
| Docker Compose | v2 or later |

---

## Architecture

The project follows **Hexagonal Architecture** (ports and adapters). The core contains all business logic and knows nothing about HTTP, MongoDB, or any external library. Everything outside the core is an adapter that plugs into a port.

```
                    ┌─────────────────────────┐
   REST (Echo) ────►│   inbound port          │
                    │   port.UserService      │
   gRPC (future) ──►│                         │
                    ├─────────────────────────┤
                    │                         │
                    │   DOMAIN + SERVICE      │  no knowledge of
                    │   (the hexagon core)    │  Echo, MongoDB, JWT
                    │                         │
                    ├─────────────────────────┤
                    │   outbound ports        │
                    │   port.UserRepository   │───► MongoDB adapter
                    │   port.TokenManager     │───► in-memory adapter
                    │                         │───► JWT manager
                    └─────────────────────────┘
```

### Dependency direction

```
rest ──► port.UserService ◄── service ──► port.UserRepository ◄── mongo
                                     │                        ◄── memory
                                     └──► port.TokenManager   ◄── pkg/jwt
```

Every arrow points **towards** the core. No layer depends on a concrete implementation of another layer — this is dependency inversion in practice.

### Verifying the boundary

The core must never import an external framework or driver. This is verifiable with a single command:

```bash
go list -deps ./internal/core/... | grep -E 'mongo|echo|golang-jwt|net/http' || echo "core is clean"
```

Only one file in the entire project imports the MongoDB driver:

```bash
grep -rln 'mongo-driver' --include='*.go' internal/
# internal/adapter/outbound/mongo/user_repository.go
```

### Why this matters in practice

Because the core depends only on interfaces, the same business logic runs against MongoDB in production and against an in-memory map in tests. Adding gRPC later means adding `internal/adapter/inbound/grpc/` without touching a single line of the core.

---

## Project Structure

```
backend-challenge/
├── cmd/
│   └── api/
│       └── main.go                    composition root — wires everything together
│
├── internal/
│   ├── core/                          THE HEXAGON — no external imports allowed
│   │   ├── domain/
│   │   │   ├── user.go                User entity (no json/bson tags)
│   │   │   └── errors.go              business errors
│   │   ├── port/
│   │   │   ├── repository.go          outbound port — what storage must provide
│   │   │   ├── service.go             inbound port — what the world can ask for
│   │   │   └── token.go               outbound port — token generation
│   │   └── service/
│   │       ├── user_service.go        all business rules
│   │       └── user_counter.go        background job (requirement 6)
│   │
│   └── adapter/
│       ├── inbound/
│       │   └── rest/
│       │       ├── dto.go             request/response shapes + mapping
│       │       ├── user_handler.go    HTTP handlers + error mapping
│       │       └── router.go          route registration
│       └── outbound/
│           ├── mongo/
│           │   └── user_repository.go the only file that knows MongoDB
│           └── memory/
│               └── user_repository.go in-memory implementation for tests
│
├── pkg/                               reusable technical utilities
│   ├── config/                        environment configuration
│   ├── mongodb/                       MongoDB connection helper
│   ├── jwt/                           HS256 token manager
│   ├── hash/                          bcrypt password hashing
│   ├── middleware/                    logging + JWT authentication
│   └── validator/                     request validation
│
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── .env.example
└── README.md
```

---

## Configuration

All configuration is read from environment variables. Copy `.env.example` to `.env` and adjust.

| Variable | Default | Description |
| --- | --- | --- |
| `APP_PORT` | `8080` | HTTP port |
| `MONGO_URI` | `mongodb://localhost:27017` | MongoDB connection string |
| `MONGO_DB` | `userdb` | Database name |
| `JWT_SECRET` | **none — required** | HMAC signing key |
| `JWT_EXPIRE_MINUTES` | `60` | Token lifetime |
| `USER_COUNT_INTERVAL_SECONDS` | `10` | Background counter interval |

`JWT_SECRET` deliberately has no default. The application refuses to start without it, which is safer than silently running with a guessable key.

When connecting from the host machine the URI needs `?authSource=admin`, because the root account created by the MongoDB image lives in the `admin` database, not in `userdb`:

```
mongodb://root:example@localhost:27017/?authSource=admin
```

Inside Docker Compose the host is the service name, not localhost:

```
mongodb://root:example@mongo:27017/?authSource=admin
```

---

## API Reference

Base path: `/api/v1`

| Method | Endpoint | Auth | Description |
| --- | --- | --- | --- |
| GET | `/health` | — | Health check |
| POST | `/api/v1/auth/register` | — | Register a new account |
| POST | `/api/v1/auth/login` | — | Authenticate and receive a JWT |
| POST | `/api/v1/users` | Bearer | Create a user |
| GET | `/api/v1/users` | Bearer | List all users |
| GET | `/api/v1/users/:id` | Bearer | Fetch a user by ID |
| PUT | `/api/v1/users/:id` | Bearer | Update name and/or email |
| DELETE | `/api/v1/users/:id` | Bearer | Delete a user |

### Status codes

| Code | Meaning |
| --- | --- |
| 200 | Success |
| 201 | Resource created |
| 204 | Deleted, no content returned |
| 400 | Validation failed or malformed ID |
| 401 | Missing, invalid, or expired token; bad credentials |
| 404 | User not found |
| 409 | Email already exists |
| 500 | Internal error (details are never exposed to the client) |

---

## JWT Guide

### How tokens are issued

Tokens are signed with **HMAC-SHA256 (HS256)** using the key from `JWT_SECRET`.

Claims included in every token:

| Claim | Meaning |
| --- | --- |
| `uid` | User ID |
| `email` | User email |
| `sub` | Subject — same as the user ID |
| `iss` | Issuer — `backend-challenge` |
| `iat` | Issued at |
| `exp` | Expiry — issue time plus `JWT_EXPIRE_MINUTES` |

### Obtaining a token

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"kraiwit@mail.com","password":"secret1234"}'
```

### Using a token

Send it in the `Authorization` header:

```bash
curl http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

The scheme is matched case-insensitively, so `bearer`, `Bearer`, and `BEARER` are all accepted, per RFC 7235.

### Capturing the token in a shell variable

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"kraiwit@mail.com","password":"secret1234"}' \
  | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')

curl http://localhost:8080/api/v1/users -H "Authorization: Bearer $TOKEN"
```

### Security measures

**Algorithm confusion is explicitly prevented.** Token verification checks that the signing method is HMAC *and* restricts accepted algorithms to HS256:

```go
jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
    if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, ErrInvalidToken
    }
    return m.secret, nil
}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
```

Without both checks, an attacker could craft a token with `{"alg":"none"}` and no signature, or — in a system that also supports RS256 — sign a token with the public key and have the server verify it as an HMAC secret. A unit test (`TestVerify_RejectsNoneAlgorithm`) asserts that such tokens are rejected.

**Verification lives outside the core.** `port.TokenManager` declares only `Generate`, because the business logic never needs to validate a token — that is the inbound adapter's responsibility. This keeps the interface minimal.

---

## Sample Requests and Responses

### Register

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Kraiwit","email":"Kraiwit@Mail.com","password":"secret1234"}'
```

`201 Created`

```json
{
  "id": "6aa3a7c77befd80066d24b45",
  "name": "Kraiwit",
  "email": "kraiwit@mail.com",
  "created_at": "2026-09-11T07:03:35.85Z"
}
```

Note that the email was submitted in mixed case and stored in lowercase. The password is never present in any response.

### Register — duplicate email

Submitting the same address in a different case is still rejected.

`409 Conflict`

```json
{ "message": "email already exists" }
```

### Register — validation failure

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"A","email":"not-an-email","password":"123"}'
```

`400 Bad Request`

```json
{ "message": "name must be at least 2 characters; email must be a valid email address; password must be at least 8 characters" }
```

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"kraiwit@mail.com","password":"secret1234"}'
```

`200 OK`

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_at": "2026-09-11T08:03:35Z",
  "user": {
    "id": "6aa3a7c77befd80066d24b45",
    "name": "Kraiwit",
    "email": "kraiwit@mail.com",
    "created_at": "2026-09-11T07:03:35.85Z"
  }
}
```

### Login — wrong credentials

`401 Unauthorized`

```json
{ "message": "invalid email or password" }
```

The same message is returned whether the email does not exist or the password is wrong, so the endpoint cannot be used to discover which addresses are registered.

### List users

```bash
curl http://localhost:8080/api/v1/users -H "Authorization: Bearer $TOKEN"
```

`200 OK`

```json
{
  "data": [
    {
      "id": "6aa3a7c77befd80066d24b45",
      "name": "Kraiwit",
      "email": "kraiwit@mail.com",
      "created_at": "2026-09-11T07:03:35.85Z"
    }
  ],
  "total": 1
}
```

### Get user by ID

```bash
curl http://localhost:8080/api/v1/users/6aa3a7c77befd80066d24b45 \
  -H "Authorization: Bearer $TOKEN"
```

`200 OK` — same shape as a single entry in `data` above.

### Update user

Either field may be omitted; omitted fields are left unchanged.

```bash
curl -X PUT http://localhost:8080/api/v1/users/6aa3a7c77befd80066d24b45 \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Kraiwit W."}'
```

`200 OK`

```json
{
  "id": "6aa3a7c77befd80066d24b45",
  "name": "Kraiwit W.",
  "email": "kraiwit@mail.com",
  "created_at": "2026-09-11T07:03:35.85Z"
}
```

### Delete user

```bash
curl -X DELETE http://localhost:8080/api/v1/users/6aa3a7c77befd80066d24b45 \
  -H "Authorization: Bearer $TOKEN"
```

`204 No Content`

### Missing token

```bash
curl -i http://localhost:8080/api/v1/users
```

`401 Unauthorized`

```json
{ "message": "missing authorization header" }
```

### Malformed ID

```bash
curl -i http://localhost:8080/api/v1/users/abc -H "Authorization: Bearer $TOKEN"
```

`400 Bad Request` — not a 500. An ID that is not a valid 24-character hex string is a client error, and the MongoDB adapter converts the parsing failure into `domain.ErrInvalidID`.

---

## Testing

```bash
go test ./...              # all tests
go test ./... -race        # with the race detector
go test ./... -cover       # with coverage
make cover                 # coverage summary
```

**Tests require no running database.** The entire suite passes with MongoDB stopped:

```bash
docker compose stop
go test ./...
docker compose start
```

### Coverage

| Package | Coverage | Note |
| --- | --- | --- |
| `internal/adapter/outbound/memory` | ~90% | Directly tested |
| `internal/adapter/inbound/rest` | ~61% | Handlers tested; router and some mappers are not |
| `internal/core/service` | ~60% | All business rules and security paths covered |
| `pkg/jwt` | high | Includes an algorithm-confusion attack test |
| `pkg/hash` | high | Verifies bcrypt format, random salt, case sensitivity |
| `internal/adapter/outbound/mongo` | 0% | Deliberate — see below |
| `cmd/api` | 0% | Wiring only |

### How MongoDB is mocked

Rather than a generated mock, the project ships a **second working implementation** of `port.UserRepository` that stores data in a map (`internal/adapter/outbound/memory`). Tests construct the service with that implementation instead of the MongoDB one.

This approach was chosen over a mocking library for two reasons. It holds real state, so a test can register, log in, update, and delete in sequence without declaring expectations for every call. And it demonstrates that the abstraction genuinely works — a second adapter runs the same business logic without a single change to the core.

The HTTP layer is tested the same way: a `stubService` implementing `port.UserService` is injected into the handler, so handler tests need neither a database nor a real JWT manager.

### What the tests do not cover

The MongoDB adapter is intentionally at 0%. Verifying it requires a real MongoDB instance, which would make `go test ./...` fail on any machine without Docker running. The following behaviours are therefore validated manually rather than by unit test:

- Correctness of BSON filters and `$set` documents
- The unique index on `email` being created successfully
- Conversion between `bson.ObjectID` and `string`
- `mongo.IsDuplicateKeyError` catching real duplicate-key errors

These can be confirmed directly in the database:

```bash
docker exec -it challenge-mongo mongosh -u root -p example --authenticationDatabase admin
```

```javascript
use userdb
db.users.findOne()        // password is a bcrypt hash starting with $2a$10$
db.users.getIndexes()     // uniq_email exists with unique: true

// the database rejects duplicates even when bypassing the API entirely
db.users.insertOne({ email: "kraiwit@mail.com", name: "bypass" })
// E11000 duplicate key error
```

Adding integration tests with `testcontainers-go` would be the natural next step in a production codebase.

---

## Design Decisions and Assumptions

### Email is normalised to lowercase

Both on write and on read. MongoDB compares strings byte by byte, so without normalisation `Kraiwit@Mail.com` and `kraiwit@mail.com` would be two distinct rows and the unique constraint would be satisfied technically while failing in intent. Normalising on read as well means users can sign in regardless of how their keyboard capitalised the address.

RFC 5321 does specify that the local part of an address is technically case-sensitive. In practice every major provider treats it as case-insensitive, so the usability gain outweighs the theoretical risk. This normalisation lives in the service layer, not in a handler or repository, because it is a business rule that must hold no matter which adapter the request arrives through.

Passwords and names are **not** normalised. Lowercasing a password would shrink the search space for an attacker, and names must be stored as the user typed them.

### Registration and user creation are separate endpoints

The specification lists both "user registration" (requirement 2) and "create a new user" (requirement 3). These were interpreted as two distinct use cases: `POST /auth/register` is public self-service signup, while `POST /users` is an authenticated operation for creating an account on someone else's behalf. They share the same underlying service method. A reviewer may reasonably interpret this differently.

### Uniqueness is enforced in two places

The service checks whether the email exists before inserting, and the database has a unique index on `email`. The service check exists to return a clean `409` in the ordinary case. The index exists because the service check alone is not safe: two concurrent registrations with the same address would both pass the check and then collide at insert time. **The unique index is the only guarantee that actually holds**, and `mongo.IsDuplicateKeyError` translates that collision back into the same domain error.

### Login errors are deliberately vague

An unknown email and a wrong password both return `invalid email or password`. Distinguishing them would let an attacker enumerate which addresses are registered. A unit test asserts that `domain.ErrNotFound` never leaks out of `Login`.

### Password length is capped at 72 characters

bcrypt silently ignores input beyond 72 bytes. Without an explicit maximum, a user who set a 100-character password would be able to authenticate with only the first 72 — a surprising and undocumented weakening. The validator rejects anything longer instead.

### The domain entity carries no struct tags

`domain.User` has neither `json:` nor `bson:` tags, and its `ID` is a `string` rather than a `bson.ObjectID`. Serialisation format is a concern of each adapter, so `internal/adapter/outbound/mongo` defines its own `userDocument` and `internal/adapter/inbound/rest` defines its own DTOs. The cost is a mapping function on each side; the benefit is a core that could be pointed at PostgreSQL without modification.

A side effect is that `UserResponse` has no password field **at all**, so a leak cannot be reintroduced by accidentally deleting a `json:"-"` tag.

### Internal errors are not surfaced

`mapError` translates known domain errors into specific status codes and collapses everything else into a generic `500`. Returning the underlying error text could expose collection names, query structure, or connection details. A test asserts that a `500` response body contains none of the original error message.

### `ID` format differs between adapters

The MongoDB adapter produces 24-character hex strings; the in-memory adapter produces sequential integers. ID format is a storage detail rather than a business rule, so this divergence is acceptable. It does mean the in-memory adapter never returns `ErrInvalidID`, which the MongoDB adapter returns for unparseable IDs.

### Password hashing is called directly rather than through a port

`port.TokenManager` is an interface, but `pkg/hash` is called directly from the service. The distinction is that token generation carries configuration and could plausibly be swapped for a different scheme, whereas bcrypt hashing is a pure function with no state and no I/O that never needs to be substituted in a test. Introducing a `PasswordHasher` port would be trivial if stricter purity were preferred.

### The background counter uses a dedicated `Count` method

Requirement 6 asks for a periodic log of the total number of users. Implementing this as `len(List())` would pull every document into memory every ten seconds, so `port.UserRepository` exposes `Count`, which maps to `CountDocuments` in MongoDB.

The goroutine selects on both `ctx.Done()` and the ticker channel so that it stops cleanly during shutdown, stops its ticker via `defer`, and creates a per-iteration timeout context that is cancelled immediately rather than accumulating deferred cancels inside the loop.

### Graceful shutdown

`signal.NotifyContext` converts `SIGINT` and `SIGTERM` into an ordinary cancellable context. On shutdown the background goroutine stops, the HTTP server stops accepting new connections while allowing in-flight requests up to ten seconds to complete, and the MongoDB client disconnects.

### Framework choice

The specification does not mandate an HTTP framework. Echo was chosen for familiarity and speed of development. Because it is confined to the inbound adapter and the middleware package, replacing it would not affect any business logic.

---

## Requirements Checklist

### Core requirements

| # | Requirement | Where |
| --- | --- | --- |
| 1 | User model with ID, Name, unique Email, hashed Password, CreatedAt | `internal/core/domain/user.go`, unique index in `mongo/user_repository.go` |
| 2 | Registration, authentication returning JWT, protected endpoints, middleware validation, HS256 | `pkg/jwt/`, `pkg/middleware/auth.go`, `core/service/user_service.go` |
| 3 | Create, get by ID, list, update name/email, delete | `rest/user_handler.go`, `core/service/user_service.go` |
| 4 | Official Go MongoDB driver | `mongo-driver/v2` in `adapter/outbound/mongo/` |
| 5 | Logging middleware capturing method, path, execution time | `pkg/middleware/logging.go` |
| 6 | Background goroutine logging user count every 10 seconds | `core/service/user_counter.go` |
| 7 | Unit tests with the standard `testing` package and mocked MongoDB | `*_test.go`, in-memory adapter |

### Bonus items

| Item | Status | Notes |
| --- | --- | --- |
| Containerisation | Done | Multi-stage Dockerfile, non-root user, compose with healthcheck-gated startup |
| Interface abstraction | Done | `port.UserRepository` with two working implementations |
| Input validation | Done | `go-playground/validator` with human-readable messages |
| Graceful shutdown | Done | `signal.NotifyContext`, ten-second drain |
| Hexagonal architecture | Done | Ports and adapters; boundary verifiable by command |
| gRPC support | Not implemented | Would be added as `internal/adapter/inbound/grpc/` with no change to the core |


### Postman collection

A Postman collection covering every endpoint is included at
[docs/postman_collection.json](docs/postman_collection.json).

To use it:
1. In Postman, click Import and select the file
2. Set the collection variable `baseUrl` to `http://localhost:8080`
3. Run the `login` request — a post-response script stores the JWT
   in the `token` collection variable automatically
4. Every other request inherits Bearer auth from the collection,
   so no manual token handling is needed