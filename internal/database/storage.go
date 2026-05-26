package database

import (
	"errors"

	"github.com/ludanortmun/teamback/internal/core"
)

var errNotImplemented = errors.New("storage not implemented")

// DummyStorage is a placeholder implementation of core.Storage.
// It allows compilation while the real implementation is built separately.
type DummyStorage struct{}

func NewDummyStorage() *DummyStorage {
	return &DummyStorage{}
}

func (d *DummyStorage) WriteUser(user core.User) error {
	return errNotImplemented
}

func (d *DummyStorage) ReadUser(id string) (core.User, error) {
	return core.User{}, errNotImplemented
}

func (d *DummyStorage) ReadUserByEmail(email string) (core.User, error) {
	return core.User{}, errNotImplemented
}

func (d *DummyStorage) SaveAssignment(assignment core.Assignment) error {
	return errNotImplemented
}

func (d *DummyStorage) GetAssignment(assignmentId string) (core.Assignment, error) {
	return core.Assignment{}, errNotImplemented
}

func (d *DummyStorage) ListAssignments() ([]core.Assignment, error) {
	return nil, errNotImplemented
}
