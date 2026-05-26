package database

import (
	"database/sql"
	"fmt"

	"github.com/ludanortmun/teamback/internal/auth"
	"github.com/ludanortmun/teamback/internal/core"
	_ "github.com/mattn/go-sqlite3"
)

func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", databaseURL+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return db, nil
}

// TeambackDatabase is the entry-point for DB operations and implements all necessary interfaces for auth and core logic.
type TeambackDatabase struct{}

func (t *TeambackDatabase) WriteUser(user core.User) error {
	//TODO implement me
	panic("implement me")
}

func (t *TeambackDatabase) ReadUser(id string) (core.User, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TeambackDatabase) ReadUserByEmail(email string) (core.User, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TeambackDatabase) SaveAssignment(assignment core.Assignment) error {
	//TODO implement me
	panic("implement me")
}

func (t *TeambackDatabase) GetAssignment(assignmentId string) (core.Assignment, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TeambackDatabase) ListAssignments() ([]core.Assignment, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TeambackDatabase) LinkIfNecessary(user core.User, externalUser auth.ExternalUserInfo) (auth.Identity, error) {
	//TODO implement me
	panic("implement me")
}
