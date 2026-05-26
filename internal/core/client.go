package core

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
)

type ctxCallerKeyType struct{}

var CtxCallerKey = ctxCallerKeyType{}

const (
	ErrMissingCallerKey    = "missing caller key"
	ErrUnauthorizedMsg     = "caller is not authorized to perform this action"
	ErrAssignmentNotFound  = "assignment does not exist"
	ErrInvalidContributions = "contributions must include exactly one entry per team member"
	ErrEmptyDescription    = "all contribution descriptions must be non-empty"
	ErrInvalidWeights      = "contribution weights must sum to 100"
)

// Client is the entrypoint for all Teamback operations
type Client struct {
	storage Storage
}

func NewClient(storage Storage) *Client {
	return &Client{storage: storage}
}

// AddUser adds a new user to the system. Only teachers can add users.
// It initializes the user's ID before persisting.
func (c *Client) AddUser(ctx context.Context, user User) (User, error) {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return User{}, err
	}

	user.ID = generateID()
	err = c.storage.WriteUser(user)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// CreateAssignment initializes a new team assignment and persists it.
// Only teachers can create assignments.
func (c *Client) CreateAssignment(ctx context.Context, title string, team []User) (Assignment, error) {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return Assignment{}, err
	}

	assignment := Assignment{
		ID:       generateID(),
		Team:     team,
		Title:    title,
		Feedback: []Answer{},
	}
	err = c.storage.SaveAssignment(assignment)
	if err != nil {
		return Assignment{}, err
	}

	return assignment, nil
}

// AddFeedback adds the provided feedback to the specified assignment.
// Only students on the assignment's team can add feedback.
func (c *Client) AddFeedback(ctx context.Context, assignmentId string, answer Answer) (Assignment, error) {
	caller, err := c.requireRole(ctx, RoleStudent)
	if err != nil {
		return Assignment{}, err
	}

	assignment, err := c.storage.GetAssignment(assignmentId)
	if err != nil {
		return Assignment{}, err
	}

	if !slices.ContainsFunc(assignment.Team, func(u User) bool { return u.ID == caller.ID }) {
		return Assignment{}, errors.New(ErrUnauthorizedMsg)
	}

	if err := validateAnswer(answer, assignment.Team); err != nil {
		return Assignment{}, err
	}

	answer.Author = caller
	assignment.Feedback = append(assignment.Feedback, answer)
	if err := c.storage.SaveAssignment(assignment); err != nil {
		return Assignment{}, err
	}

	return assignment, nil
}

// validateAnswer checks that the answer has valid contributions for the given team.
func validateAnswer(answer Answer, team []User) error {
	if len(answer.MemberContributions) != len(team) {
		return errors.New(ErrInvalidContributions)
	}
	for _, member := range team {
		if _, ok := answer.MemberContributions[member.ID]; !ok {
			return errors.New(ErrInvalidContributions)
		}
	}

	var totalWeight uint
	for _, contrib := range answer.MemberContributions {
		if contrib.Description == "" {
			return errors.New(ErrEmptyDescription)
		}
		totalWeight += uint(contrib.Weight)
	}
	if totalWeight != 100 {
		return errors.New(ErrInvalidWeights)
	}

	return nil
}

// GetAssignment retrieves an assignment with role-based visibility:
// - Teachers can see all answers
// - Students in the team can see only their own answer
// - Any other user is unauthorized
func (c *Client) GetAssignment(ctx context.Context, assignmentId string) (Assignment, error) {
	caller, err := c.requireAuth(ctx)
	if err != nil {
		return Assignment{}, err
	}

	assignment, err := c.storage.GetAssignment(assignmentId)
	if err != nil {
		return Assignment{}, err
	}

	if caller.Role == RoleTeacher {
		return assignment, nil
	}

	if !slices.ContainsFunc(assignment.Team, func(u User) bool { return u.ID == caller.ID }) {
		return Assignment{}, errors.New(ErrUnauthorizedMsg)
	}

	// Filter feedback to only the caller's own answer
	filtered := []Answer{}
	for _, a := range assignment.Feedback {
		if a.Author.ID == caller.ID {
			filtered = append(filtered, a)
		}
	}
	assignment.Feedback = filtered
	return assignment, nil
}

// GetAssignments lists all assignments visible to the caller.
// Teachers see all assignments with full feedback.
// Students see only assignments where they are a team member, with feedback
// filtered to only their own answer.
func (c *Client) GetAssignments(ctx context.Context) ([]Assignment, error) {
	caller, err := c.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	all, err := c.storage.ListAssignments()
	if err != nil {
		return nil, err
	}

	if caller.Role == RoleTeacher {
		return all, nil
	}

	var visible []Assignment
	for _, assignment := range all {
		if !slices.ContainsFunc(assignment.Team, func(u User) bool { return u.ID == caller.ID }) {
			continue
		}
		filtered := []Answer{}
		for _, a := range assignment.Feedback {
			if a.Author.ID == caller.ID {
				filtered = append(filtered, a)
			}
		}
		assignment.Feedback = filtered
		visible = append(visible, assignment)
	}
	return visible, nil
}

// requireRole checks if the caller has the specified role and returns the caller's user object if so.
// Otherwise, it returns an error.
func (c *Client) requireRole(ctx context.Context, role Role) (User, error) {
	caller, err := c.requireAuth(ctx)
	if err != nil {
		return User{}, err
	}
	if caller.Role != role {
		return User{}, errors.New(ErrUnauthorizedMsg)
	}
	return caller, nil
}

// requireAuth extracts and returns the authenticated caller from the context.
func (c *Client) requireAuth(ctx context.Context) (User, error) {
	callerId, ok := ctx.Value(CtxCallerKey).(string)
	if !ok {
		return User{}, errors.New(ErrMissingCallerKey)
	}

	caller, err := c.storage.ReadUser(callerId)
	if err != nil {
		return User{}, errors.New(ErrUnauthorizedMsg)
	}
	return caller, nil
}

func generateID() string {
	return uuid.New().String()
}
