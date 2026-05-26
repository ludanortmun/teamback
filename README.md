# Teamback

A simple web application for gathering feedback on group school projects. Teachers create student profiles and group assignments; students submit peer feedback visible only to the teacher, with optional comments visible to teammates.

## Tech Stack

- **Go** — Standard library HTTP server with SSR (html/template)
- **SQLite** — Local development database (Postgres-ready schema)
- **Google OIDC** — Authentication via Google accounts

## Prerequisites

- Go 1.26.2
- A Google OAuth 2.0 client (create one at [Google Cloud Console](https://console.cloud.google.com/apis/credentials))
- GCC (required by go-sqlite3 CGo dependency)

## Setup

1. Clone the repository and install dependencies:

   ```sh
   go mod download
   ```

2. Copy the example environment file and fill in your credentials:

   ```sh
   cp .env.example .env
   ```

   Edit `.env` with your Google OAuth client ID, secret, and a random session secret.

3. Run the server:

   ```sh
   go run ./cmd/server
   ```

   The server starts at `http://localhost:8080` by default.

## Commands

| Command | Description |
|---------|-------------|
| `go run ./cmd/server` | Start the development server |
| `go build -o teamback ./cmd/server` | Build a production binary |
| `go test ./...` | Run all tests |
| `go vet ./...` | Run static analysis |

## Project Structure

```
cmd/server/         — Application entrypoint
internal/config/    — Environment-based configuration
internal/database/  — Database connection and migrations
internal/auth/      — Google OIDC and session management
internal/handler/   — HTTP handlers and template rendering
internal/middleware/ — Authentication middleware
migrations/         — SQL migration files
templates/          — HTML templates
static/             — Static assets (CSS)
```

## License

MIT
