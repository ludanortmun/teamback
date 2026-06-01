# Teamback

A simple web application for gathering feedback on group school projects. Teachers create student profiles and group assignments; students submit peer feedback with contribution weights and descriptions.

## Tech Stack

- **Go** — Standard library HTTP server with SSR (html/template)
- **PostgreSQL** — Relational database for persistence
- **Docker** — Containerized deployment via Docker Compose
- **Google OIDC** — Authentication via Google accounts

## Prerequisites

- Docker and Docker Compose (recommended), **or**:
  - Go 1.26.2
  - PostgreSQL 16+
- A Google OAuth 2.0 client (create one at [Google Cloud Console](https://console.cloud.google.com/apis/credentials))

## Quick Start (Docker)

1. Copy the example environment file and fill in your credentials:

   ```sh
   cp .env.example .env.docker
   ```

   Edit `.env.docker` with your Google OAuth client ID, secret, a random session secret, and set `DATABASE_URL=postgres://teamback:teamback@postgres:5432/teamback?sslmode=disable`.

2. Start the application:

   ```sh
   docker compose up
   ```

   This builds the app image and starts both PostgreSQL and the server. The app is available at `http://localhost:8080`.

## Local Development (without Docker)

1. Install dependencies:

   ```sh
   go mod download
   ```

2. Start a PostgreSQL instance and set `DATABASE_URL` in `.env` (see `.env.example`).

3. Run the server:

   ```sh
   go run ./cmd/server
   ```

   The server starts at `http://localhost:8080` by default. Migrations run automatically on startup.

## Commands

| Command | Description |
|---------|-------------|
| `docker compose up` | Start app + database |
| `go run ./cmd/server` | Start the development server locally |
| `go build -o teamback ./cmd/server` | Build a production binary |
| `go test ./...` | Run all tests |
| `go vet ./...` | Run static analysis |

## Project Structure

```
cmd/server/         — Application entrypoint
internal/core/      — Domain models, business rules, and Client API
internal/config/    — Environment-based configuration
internal/database/  — PostgreSQL storage implementation and migrations
internal/auth/      — Google OIDC, identity linking, and session management
internal/handler/   — HTTP handlers (users, assignments, feedback)
internal/middleware/ — Authentication middleware
migrations/         — SQL migration files
templates/          — HTML templates (SSR)
static/             — Static assets (CSS)
```

## License

MIT
