package core

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
)

const (
	CtxCallerKey = iota
)

const (
	ErrMissingCallerKey   = "missing caller key"
	ErrUnauthorizedMsg    = "caller is not authorized to perform this action"
	ErrAssignmentNotFound = "assignment does not exist"
)

// Client is the entrypoint for all Teamback operations
type Client struct {
	storage Storage
}

func NewClient(storage Storage) *Client {
	return &Client{storage: storage}
}

// AddUser adds a new user to the system. Only teachers can add users.
func (c *Client) AddUser(ctx context.Context, user User) error {
	callerId, ok := ctx.Value(CtxCallerKey).(string)
	if !ok {
		return errors.New(ErrMissingCallerKey)
	}

	caller, err := c.storage.ReadUser(callerId)
	if err != nil || caller.Role != RoleTeacher {
		return errors.New(ErrUnauthorizedMsg)
	}

	return c.storage.WriteUser(user)
}

// CreateAssignment initializes a new team assignment and persists it.
// Only teachers can create assignments.
func (c *Client) CreateAssignment(ctx context.Context, title string, team []User) (Assignment, error) {
	callerId, ok := ctx.Value(CtxCallerKey).(string)
	if !ok {
		return Assignment{}, errors.New(ErrMissingCallerKey)
	}

	caller, err := c.storage.ReadUser(callerId)
	if err != nil || caller.Role != RoleTeacher {
		return Assignment{}, errors.New(ErrUnauthorizedMsg)
	}

	assignment := Assignment{
		ID:       generateID(),
		Team:     team,
		Title:    title,
		Feedback: []Answer{},
	}
	err = c.storage.SaveAssignment(assignment)

	return assignment, nil
}

// AddFeedback adds the provided feedback to the specified assignment. Only students on the assignment's team can add feedback.
func (c *Client) AddFeedback(ctx context.Context, assignmentId string, answer Answer) (Assignment, error) {
	callerId, ok := ctx.Value(CtxCallerKey).(string)
	if !ok {
		return Assignment{}, errors.New(ErrMissingCallerKey)
	}
	caller, err := c.storage.ReadUser(callerId)
	if err != nil {
		return Assignment{}, errors.New(ErrUnauthorizedMsg)
	}

	assignment, err := c.storage.GetAssignment(assignmentId)
	if err != nil {
		return Assignment{}, errors.New(ErrAssignmentNotFound)
	}

	if !slices.ContainsFunc(assignment.Team, func(u User) bool { return u.ID == caller.ID }) {
		return Assignment{}, errors.New(ErrUnauthorizedMsg)
	}

	return Assignment{}, nil
}

func generateID() string {
	return uuid.New().String()
}
