package core

// Storage handles persistence for users and assignments.
type Storage interface {
	WriteUser(user User) error
	ReadUser(id string) (User, error)
	ReadUserByEmail(email string) (User, error)
	ListStudents() ([]User, error)
	DeleteUser(id string) error
	SaveAssignment(assignment Assignment) error
	DeleteAssignment(id string) error
	GetAssignment(assignmentId string) (Assignment, error)
	ListAssignments() ([]Assignment, error)
	// ListFeedbackHistory returns all feedback versions for a given author on an assignment (newest first).
	ListFeedbackHistory(assignmentID string, authorID string) ([]Answer, error)
	SaveAnswerVersion(assignmentID string, answer Answer) error
}

// SummaryStorage handles persistence for AI-generated assignment summaries.
type SummaryStorage interface {
	CreateSummary(assignmentID string) (AssignmentSummary, error)
	CompleteSummary(id string, summary string, attentionRequired bool) error
	FailSummary(id string) error
	GetLatestSummary(assignmentID string) (AssignmentSummary, error)
	GetLatestSummaries(assignmentIDs []string) ([]AssignmentSummary, error)
	ListSummaries(assignmentID string) ([]AssignmentSummary, error)
	// GetRetryableSummaries returns failed summaries with attempts < maxAttempts
	// that have not been superseded by a newer summary for the same assignment.
	GetRetryableSummaries(maxAttempts int) ([]AssignmentSummary, error)
	// ResetForRetry increments attempts and sets status back to pending.
	ResetForRetry(id string) error
}

// Summarizer generates AI summaries from assignment feedback.
type Summarizer interface {
	Summarize(assignment Assignment, feedback []Answer) (SummaryResult, error)
}
