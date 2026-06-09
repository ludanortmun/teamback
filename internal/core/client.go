package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/google/uuid"
)

type ctxCallerKeyType struct{}

var CtxCallerKey = ctxCallerKeyType{}

const (
	ErrMissingCallerKey     = "missing caller key"
	ErrUnauthorizedMsg      = "caller is not authorized to perform this action"
	ErrAssignmentNotFound   = "assignment does not exist"
	ErrInvalidContributions = "contributions must include exactly one entry per team member"
	ErrEmptyDescription     = "all contribution descriptions must be non-empty"
	ErrInvalidWeights       = "contribution weights must sum to 100"
	ErrUserNotFound         = "user not found"
)

// Client is the entrypoint for all Teamback operations
type Client struct {
	storage        Storage
	summaryStorage SummaryStorage
	summarizer     Summarizer
	workerSubmit   func(func())
}

type ClientOption func(*Client)

func WithSummarizer(s Summarizer, ss SummaryStorage, submit func(func())) ClientOption {
	return func(c *Client) {
		c.summarizer = s
		c.summaryStorage = ss
		c.workerSubmit = submit
	}
}

func NewClient(storage Storage, opts ...ClientOption) *Client {
	c := &Client{storage: storage}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetUserByEmail retrieves a user by their email address.
// Any authenticated user can look up another user by email.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (User, error) {
	_, err := c.requireAuth(ctx)
	if err != nil {
		return User{}, err
	}

	user, err := c.storage.ReadUserByEmail(email)
	if err != nil {
		return User{}, errors.New(ErrUserNotFound)
	}
	return user, nil
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

// ListStudents returns all student users. Teacher-only.
func (c *Client) ListStudents(ctx context.Context) ([]User, error) {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return nil, err
	}
	return c.storage.ListStudents()
}

// DeleteUser deletes a student user. Teacher-only.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return err
	}
	return c.storage.DeleteUser(id)
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

// DeleteAssignment deletes an assignment and all related data. Teacher-only.
func (c *Client) DeleteAssignment(ctx context.Context, id string) error {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return err
	}
	return c.storage.DeleteAssignment(id)
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
	if index := slices.IndexFunc(assignment.Feedback, func(existing Answer) bool {
		return existing.Author.ID == caller.ID
	}); index >= 0 {
		assignment.Feedback[index] = answer
	} else {
		assignment.Feedback = append(assignment.Feedback, answer)
	}
	if err := c.storage.SaveAssignment(assignment); err != nil {
		return Assignment{}, err
	}
	if err := c.storage.SaveAnswerVersion(assignmentId, answer); err != nil {
		return Assignment{}, err
	}

	// Trigger async AI summary generation
	c.triggerSummary(assignmentId)

	return assignment, nil
}

// triggerSummary starts an asynchronous AI summary job for the assignment.
func (c *Client) triggerSummary(assignmentID string) {
	if c.summarizer == nil || c.summaryStorage == nil || c.workerSubmit == nil {
		return
	}

	summary, err := c.summaryStorage.CreateSummary(assignmentID)
	if err != nil {
		log.Printf("ERROR: failed to create summary record for assignment %s: %v", assignmentID, err)
		return
	}

	c.workerSubmit(func() {
		c.executeSummary(summary.ID, assignmentID)
	})
}

// TriggerSummarySync creates a new summary record and executes the AI summarization synchronously.
// Returns the completed summary or an error.
func (c *Client) TriggerSummarySync(assignmentID string) (AssignmentSummary, error) {
	if c.summarizer == nil || c.summaryStorage == nil {
		return AssignmentSummary{}, errors.New("AI features are disabled")
	}

	summary, err := c.summaryStorage.CreateSummary(assignmentID)
	if err != nil {
		return AssignmentSummary{}, fmt.Errorf("creating summary record: %w", err)
	}

	c.executeSummary(summary.ID, assignmentID)

	latest, err := c.summaryStorage.GetLatestSummary(assignmentID)
	if err != nil {
		return AssignmentSummary{}, fmt.Errorf("fetching completed summary: %w", err)
	}
	return latest, nil
}

// executeSummary runs the AI summarizer for a given summary record.
func (c *Client) executeSummary(summaryID string, assignmentID string) {
	assignment, err := c.storage.GetAssignment(assignmentID)
	if err != nil {
		log.Printf("ERROR: failed to load assignment %s for summary: %v", assignmentID, err)
		_ = c.summaryStorage.FailSummary(summaryID)
		return
	}

	result, err := c.summarizer.Summarize(assignment, assignment.Feedback)
	if err != nil {
		log.Printf("ERROR: AI summary failed for assignment %s: %v", assignmentID, err)
		_ = c.summaryStorage.FailSummary(summaryID)
		return
	}

	if err := c.summaryStorage.CompleteSummary(summaryID, result.Summary, result.AttentionRequired); err != nil {
		log.Printf("ERROR: failed to save summary for assignment %s: %v", assignmentID, err)
	}
}

// RetryFailedSummaries finds failed summaries eligible for retry and resubmits them.
func (c *Client) RetryFailedSummaries(maxAttempts int) {
	if c.summarizer == nil || c.summaryStorage == nil || c.workerSubmit == nil {
		return
	}

	retryable, err := c.summaryStorage.GetRetryableSummaries(maxAttempts)
	if err != nil {
		log.Printf("ERROR: failed to get retryable summaries: %v", err)
		return
	}

	for _, s := range retryable {
		summaryID := s.ID
		assignmentID := s.AssignmentID

		if err := c.summaryStorage.ResetForRetry(summaryID); err != nil {
			log.Printf("ERROR: failed to reset summary %s for retry: %v", summaryID, err)
			continue
		}

		c.workerSubmit(func() {
			c.executeSummary(summaryID, assignmentID)
		})
	}
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

// GetAssignmentSummaries returns the latest AI summaries for the given assignments (teacher-only).
func (c *Client) GetAssignmentSummaries(ctx context.Context, assignmentIDs []string) (map[string]AssignmentSummary, error) {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return nil, err
	}

	if c.summaryStorage == nil {
		return nil, errors.New("AI features are disabled")
	}

	summaries, err := c.summaryStorage.GetLatestSummaries(assignmentIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[string]AssignmentSummary, len(summaries))
	for _, s := range summaries {
		result[s.AssignmentID] = s
	}
	return result, nil
}

// GetAssignmentSummary returns the latest AI summary for the assignment (teacher-only).
func (c *Client) GetAssignmentSummary(ctx context.Context, assignmentID string) (AssignmentSummary, error) {
	summaries, err := c.GetAssignmentSummaries(ctx, []string{assignmentID})
	if err != nil {
		return AssignmentSummary{}, err
	}
	s, ok := summaries[assignmentID]
	if !ok {
		return AssignmentSummary{}, errors.New("no summary found for assignment " + assignmentID)
	}
	return s, nil
}

// ListAssignmentSummaries returns all summary snapshots for an assignment (teacher-only).
func (c *Client) ListAssignmentSummaries(ctx context.Context, assignmentID string) ([]AssignmentSummary, error) {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return nil, err
	}

	if c.summaryStorage == nil {
		return nil, errors.New("AI features are disabled")
	}

	return c.summaryStorage.ListSummaries(assignmentID)
}

// ListFeedbackHistory returns all feedback versions for a given author on an assignment (teacher-only).
func (c *Client) ListFeedbackHistory(ctx context.Context, assignmentID string, authorID string) ([]Answer, error) {
	_, err := c.requireRole(ctx, RoleTeacher)
	if err != nil {
		return nil, err
	}

	return c.storage.ListFeedbackHistory(assignmentID, authorID)
}
