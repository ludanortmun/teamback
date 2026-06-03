package core

// Storage handles persistence for users and assignments.
type Storage interface {
	WriteUser(user User) error
	ReadUser(id string) (User, error)
	ReadUserByEmail(email string) (User, error)
	SaveAssignment(assignment Assignment) error
	GetAssignment(assignmentId string) (Assignment, error)
	ListAssignments() ([]Assignment, error)
	// ListFeedbackHistory returns all feedback versions for a given author on an assignment (newest first).
	ListFeedbackHistory(assignmentID string, authorID string) ([]Answer, error)
}

// SummaryStorage handles persistence for AI-generated assignment summaries.
type SummaryStorage interface {
	CreateSummary(assignmentID string) (AssignmentSummary, error)
	CompleteSummary(id string, summary string) error
	FailSummary(id string) error
	GetLatestSummary(assignmentID string) (AssignmentSummary, error)
	ListSummaries(assignmentID string) ([]AssignmentSummary, error)
}

// Summarizer generates AI summaries from assignment feedback.
type Summarizer interface {
	Summarize(assignment Assignment, feedback []Answer) (string, error)
}
