package database_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ludanortmun/teamback/internal/auth"
	"github.com/ludanortmun/teamback/internal/core"
	"github.com/ludanortmun/teamback/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) *database.TeambackDatabase {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("teamback_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting connection string: %v", err)
	}

	db, err := database.Open(connStr)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Run migrations
	_, thisFile, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
	if err := database.Migrate(db, migrationsPath); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	return database.NewTeambackDatabase(db)
}

func TestWriteAndReadUser(t *testing.T) {
	tdb := setupTestDB(t)

	user := core.User{
		ID:    "user-1",
		Name:  "Alice",
		Email: "alice@example.com",
		Role:  core.RoleStudent,
	}

	if err := tdb.WriteUser(user); err != nil {
		t.Fatalf("WriteUser: %v", err)
	}

	got, err := tdb.ReadUser("user-1")
	if err != nil {
		t.Fatalf("ReadUser: %v", err)
	}
	if got != user {
		t.Errorf("got %+v, want %+v", got, user)
	}
}

func TestWriteUserIdempotent(t *testing.T) {
	tdb := setupTestDB(t)

	user := core.User{ID: "user-1", Name: "Alice", Email: "alice@example.com", Role: core.RoleStudent}
	if err := tdb.WriteUser(user); err != nil {
		t.Fatalf("first WriteUser: %v", err)
	}

	// Update name — same ID should upsert
	user.Name = "Alice Updated"
	if err := tdb.WriteUser(user); err != nil {
		t.Fatalf("second WriteUser: %v", err)
	}

	got, err := tdb.ReadUser("user-1")
	if err != nil {
		t.Fatalf("ReadUser: %v", err)
	}
	if got.Name != "Alice Updated" {
		t.Errorf("name not updated: got %q", got.Name)
	}
}

func TestReadUserByEmail(t *testing.T) {
	tdb := setupTestDB(t)

	user := core.User{ID: "user-1", Name: "Bob", Email: "bob@example.com", Role: core.RoleTeacher}
	if err := tdb.WriteUser(user); err != nil {
		t.Fatalf("WriteUser: %v", err)
	}

	got, err := tdb.ReadUserByEmail("bob@example.com")
	if err != nil {
		t.Fatalf("ReadUserByEmail: %v", err)
	}
	if got != user {
		t.Errorf("got %+v, want %+v", got, user)
	}
}

func TestReadUserByEmailNotFound(t *testing.T) {
	tdb := setupTestDB(t)

	_, err := tdb.ReadUserByEmail("nobody@example.com")
	if err == nil {
		t.Fatal("expected error for non-existent email")
	}
}

func TestEmailUniquenessConstraint(t *testing.T) {
	tdb := setupTestDB(t)

	u1 := core.User{ID: "user-1", Name: "Alice", Email: "same@example.com", Role: core.RoleStudent}
	u2 := core.User{ID: "user-2", Name: "Bob", Email: "same@example.com", Role: core.RoleStudent}

	if err := tdb.WriteUser(u1); err != nil {
		t.Fatalf("WriteUser u1: %v", err)
	}
	if err := tdb.WriteUser(u2); err == nil {
		t.Fatal("expected error for duplicate email with different ID")
	}
}

func TestSaveAndGetAssignment(t *testing.T) {
	tdb := setupTestDB(t)

	// Create team members first
	alice := core.User{ID: "user-1", Name: "Alice", Email: "alice@test.com", Role: core.RoleStudent}
	bob := core.User{ID: "user-2", Name: "Bob", Email: "bob@test.com", Role: core.RoleStudent}
	if err := tdb.WriteUser(alice); err != nil {
		t.Fatalf("WriteUser alice: %v", err)
	}
	if err := tdb.WriteUser(bob); err != nil {
		t.Fatalf("WriteUser bob: %v", err)
	}

	assignment := core.Assignment{
		ID:    "assign-1",
		Title: "Sprint 1",
		Team:  []core.User{alice, bob},
		Feedback: []core.Answer{
			{
				Author: alice,
				MemberContributions: map[string]core.Contribution{
					"user-1": {Description: "Did frontend", Weight: 50},
					"user-2": {Description: "Did backend", Weight: 50},
				},
			},
		},
	}

	if err := tdb.SaveAssignment(assignment); err != nil {
		t.Fatalf("SaveAssignment: %v", err)
	}

	got, err := tdb.GetAssignment("assign-1")
	if err != nil {
		t.Fatalf("GetAssignment: %v", err)
	}

	if got.ID != "assign-1" || got.Title != "Sprint 1" {
		t.Errorf("unexpected assignment: %+v", got)
	}
	if len(got.Team) != 2 {
		t.Errorf("expected 2 team members, got %d", len(got.Team))
	}
	if len(got.Feedback) != 1 {
		t.Errorf("expected 1 answer, got %d", len(got.Feedback))
	}
	if len(got.Feedback[0].MemberContributions) != 2 {
		t.Errorf("expected 2 contributions, got %d", len(got.Feedback[0].MemberContributions))
	}
}

func TestSaveAssignmentIdempotent(t *testing.T) {
	tdb := setupTestDB(t)

	alice := core.User{ID: "user-1", Name: "Alice", Email: "alice@test.com", Role: core.RoleStudent}
	if err := tdb.WriteUser(alice); err != nil {
		t.Fatalf("WriteUser: %v", err)
	}

	assignment := core.Assignment{
		ID:    "assign-1",
		Title: "Sprint 1",
		Team:  []core.User{alice},
	}

	if err := tdb.SaveAssignment(assignment); err != nil {
		t.Fatalf("first SaveAssignment: %v", err)
	}

	// Save again with updated title
	assignment.Title = "Sprint 1 Updated"
	if err := tdb.SaveAssignment(assignment); err != nil {
		t.Fatalf("second SaveAssignment: %v", err)
	}

	got, err := tdb.GetAssignment("assign-1")
	if err != nil {
		t.Fatalf("GetAssignment: %v", err)
	}
	if got.Title != "Sprint 1 Updated" {
		t.Errorf("title not updated: got %q", got.Title)
	}
}

func TestListAssignments(t *testing.T) {
	tdb := setupTestDB(t)

	alice := core.User{ID: "user-1", Name: "Alice", Email: "alice@test.com", Role: core.RoleStudent}
	if err := tdb.WriteUser(alice); err != nil {
		t.Fatalf("WriteUser: %v", err)
	}

	if err := tdb.SaveAssignment(core.Assignment{ID: "a1", Title: "First", Team: []core.User{alice}}); err != nil {
		t.Fatalf("SaveAssignment a1: %v", err)
	}
	if err := tdb.SaveAssignment(core.Assignment{ID: "a2", Title: "Second", Team: []core.User{alice}}); err != nil {
		t.Fatalf("SaveAssignment a2: %v", err)
	}

	assignments, err := tdb.ListAssignments()
	if err != nil {
		t.Fatalf("ListAssignments: %v", err)
	}
	if len(assignments) != 2 {
		t.Errorf("expected 2 assignments, got %d", len(assignments))
	}
}

func TestLinkIfNecessary(t *testing.T) {
	tdb := setupTestDB(t)

	user := core.User{ID: "user-1", Name: "Alice", Email: "alice@test.com", Role: core.RoleStudent}
	if err := tdb.WriteUser(user); err != nil {
		t.Fatalf("WriteUser: %v", err)
	}

	extUser := auth.ExternalUserInfo{Sub: "google-123", Email: "alice@gmail.com", Name: "Alice G"}

	identity, err := tdb.LinkIfNecessary(user, extUser)
	if err != nil {
		t.Fatalf("LinkIfNecessary: %v", err)
	}
	if identity.UserID != "user-1" || identity.ExternalID != "google-123" {
		t.Errorf("unexpected identity: %+v", identity)
	}
}

func TestLinkIfNecessaryIdempotent(t *testing.T) {
	tdb := setupTestDB(t)

	user := core.User{ID: "user-1", Name: "Alice", Email: "alice@test.com", Role: core.RoleStudent}
	if err := tdb.WriteUser(user); err != nil {
		t.Fatalf("WriteUser: %v", err)
	}

	extUser := auth.ExternalUserInfo{Sub: "google-123", Email: "alice@gmail.com", Name: "Alice G"}

	// Link twice — should not error
	if _, err := tdb.LinkIfNecessary(user, extUser); err != nil {
		t.Fatalf("first LinkIfNecessary: %v", err)
	}
	identity, err := tdb.LinkIfNecessary(user, extUser)
	if err != nil {
		t.Fatalf("second LinkIfNecessary: %v", err)
	}
	if identity.UserID != "user-1" {
		t.Errorf("unexpected identity after idempotent link: %+v", identity)
	}
}

func TestGetAssignmentNotFound(t *testing.T) {
	tdb := setupTestDB(t)

	_, err := tdb.GetAssignment("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent assignment")
	}
}
