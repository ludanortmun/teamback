package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ludanortmun/teamback/internal/auth"
	"github.com/ludanortmun/teamback/internal/config"
	"github.com/ludanortmun/teamback/internal/core"
	"github.com/ludanortmun/teamback/internal/database"
	"github.com/ludanortmun/teamback/internal/handler"
	"github.com/ludanortmun/teamback/internal/middleware"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	sessions := auth.NewSessionStore(cfg.SessionSecret)

	storage := database.NewDummyStorage()
	client := core.NewClient(storage)

	h, err := handler.New(client, "templates")
	if err != nil {
		return fmt.Errorf("initializing handlers: %w", err)
	}

	oidc := auth.NewOIDCHandler(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleRedirectURL,
		sessions,
		makeLoginCallback(db),
	)

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public routes
	mux.HandleFunc("GET /{$}", h.HandleHome)
	mux.HandleFunc("GET /login", h.HandleLogin)

	// Auth routes
	mux.HandleFunc("GET /auth/login", oidc.HandleLogin)
	mux.HandleFunc("GET /auth/callback", oidc.HandleCallback)
	mux.HandleFunc("GET /auth/logout", oidc.HandleLogout)

	// Protected routes (require authentication)
	authMw := middleware.RequireAuth(sessions)

	mux.Handle("GET /assignments", authMw(http.HandlerFunc(h.HandleListAssignments)))
	mux.Handle("GET /assignments/new", authMw(http.HandlerFunc(h.HandleNewAssignment)))
	mux.Handle("POST /assignments", authMw(http.HandlerFunc(h.HandleCreateAssignment)))
	mux.Handle("GET /assignments/{id}", authMw(http.HandlerFunc(h.HandleViewAssignment)))
	mux.Handle("GET /assignments/{id}/feedback", authMw(http.HandlerFunc(h.HandleNewFeedback)))
	mux.Handle("POST /assignments/{id}/feedback", authMw(http.HandlerFunc(h.HandleCreateFeedback)))
	mux.Handle("GET /users/new", authMw(http.HandlerFunc(h.HandleNewUser)))
	mux.Handle("POST /users", authMw(http.HandlerFunc(h.HandleCreateUser)))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		log.Printf("Server starting on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// makeLoginCallback returns the function called after successful Google auth.
// It links the Google account to an existing user by email.
func makeLoginCallback(db *sql.DB) func(ctx context.Context, email, name, googleSub string) (string, error) {
	return func(ctx context.Context, email, name, googleSub string) (string, error) {
		var userID string
		err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE email = ?", email).Scan(&userID)
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not registered: %s", email)
		}
		if err != nil {
			return "", fmt.Errorf("querying user: %w", err)
		}

		// Link Google sub if not already linked
		_, err = db.ExecContext(ctx,
			"UPDATE users SET google_sub = ? WHERE id = ? AND google_sub IS NULL",
			googleSub, userID,
		)
		if err != nil {
			return "", fmt.Errorf("linking google account: %w", err)
		}

		return userID, nil
	}
}
