package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ludanortmun/teamback/internal/auth"
	"github.com/ludanortmun/teamback/internal/core"
)

// Open connects to a PostgreSQL database using the given DSN.
func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return db, nil
}

// TeambackDatabase is the entry-point for DB operations and implements all necessary interfaces for auth and core logic.
type TeambackDatabase struct {
	db *sql.DB
}

func NewTeambackDatabase(db *sql.DB) *TeambackDatabase {
	return &TeambackDatabase{db: db}
}

func (t *TeambackDatabase) WriteUser(user core.User) error {
	roleStr := "student"
	if user.Role == core.RoleTeacher {
		roleStr = "teacher"
	}
	_, err := t.db.Exec(
		`INSERT INTO users (id, name, email, role)
		 VALUES ($1, $2, LOWER($3), $4)
		 ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, email = LOWER(EXCLUDED.email), role = EXCLUDED.role`,
		user.ID, user.Name, user.Email, roleStr,
	)
	if err != nil {
		return fmt.Errorf("writing user: %w", err)
	}
	return nil
}

func (t *TeambackDatabase) ReadUser(id string) (core.User, error) {
	var user core.User
	var roleStr string
	err := t.db.QueryRow(
		`SELECT id, name, email, role FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Name, &user.Email, &roleStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.User{}, fmt.Errorf("user %s not found", id)
		}
		return core.User{}, fmt.Errorf("reading user: %w", err)
	}
	user.Role = parseRole(roleStr)
	return user, nil
}

func (t *TeambackDatabase) ReadUserByEmail(email string) (core.User, error) {
	var user core.User
	var roleStr string
	err := t.db.QueryRow(
		`SELECT id, name, email, role FROM users WHERE LOWER(email) = LOWER($1)`, email,
	).Scan(&user.ID, &user.Name, &user.Email, &roleStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.User{}, fmt.Errorf("user with email %s not registered", email)
		}
		return core.User{}, fmt.Errorf("reading user by email: %w", err)
	}
	user.Role = parseRole(roleStr)
	return user, nil
}

func (t *TeambackDatabase) SaveAssignment(assignment core.Assignment) error {
	tx, err := t.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Upsert assignment
	_, err = tx.Exec(
		`INSERT INTO assignments (id, title)
		 VALUES ($1, $2)
		 ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title`,
		assignment.ID, assignment.Title,
	)
	if err != nil {
		return fmt.Errorf("upserting assignment: %w", err)
	}

	// Sync team members: delete old, insert current
	_, err = tx.Exec(`DELETE FROM assignment_members WHERE assignment_id = $1`, assignment.ID)
	if err != nil {
		return fmt.Errorf("clearing assignment members: %w", err)
	}
	for _, member := range assignment.Team {
		_, err = tx.Exec(
			`INSERT INTO assignment_members (assignment_id, user_id) VALUES ($1, $2)`,
			assignment.ID, member.ID,
		)
		if err != nil {
			return fmt.Errorf("inserting assignment member: %w", err)
		}
	}

	// Insert answers as new versions (never overwrite)
	for _, answer := range assignment.Feedback {
		var nextVersion int
		err = tx.QueryRow(
			`SELECT COALESCE(MAX(version), 0) + 1 FROM answers WHERE assignment_id = $1 AND author_id = $2`,
			assignment.ID, answer.Author.ID,
		).Scan(&nextVersion)
		if err != nil {
			return fmt.Errorf("getting next version: %w", err)
		}

		answerID := fmt.Sprintf("%s:%s:%d", assignment.ID, answer.Author.ID, nextVersion)
		_, err = tx.Exec(
			`INSERT INTO answers (id, assignment_id, author_id, version) VALUES ($1, $2, $3, $4)`,
			answerID, assignment.ID, answer.Author.ID, nextVersion,
		)
		if err != nil {
			return fmt.Errorf("inserting answer: %w", err)
		}
		for memberID, contrib := range answer.MemberContributions {
			_, err = tx.Exec(
				`INSERT INTO contributions (answer_id, member_id, description, weight) VALUES ($1, $2, $3, $4)`,
				answerID, memberID, contrib.Description, contrib.Weight,
			)
			if err != nil {
				return fmt.Errorf("inserting contribution: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (t *TeambackDatabase) GetAssignment(assignmentId string) (core.Assignment, error) {
	var assignment core.Assignment
	err := t.db.QueryRow(
		`SELECT id, title FROM assignments WHERE id = $1`, assignmentId,
	).Scan(&assignment.ID, &assignment.Title)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.Assignment{}, fmt.Errorf("assignment %s not found", assignmentId)
		}
		return core.Assignment{}, fmt.Errorf("reading assignment: %w", err)
	}

	// Load team members
	team, err := t.loadAssignmentTeam(assignmentId)
	if err != nil {
		return core.Assignment{}, err
	}
	assignment.Team = team

	// Load feedback
	feedback, err := t.loadAssignmentFeedback(assignmentId)
	if err != nil {
		return core.Assignment{}, err
	}
	assignment.Feedback = feedback

	return assignment, nil
}

func (t *TeambackDatabase) ListAssignments() ([]core.Assignment, error) {
	// TODO: Replace per-assignment team/feedback loads with batched JOIN-based loading to avoid N+1 queries.
	rows, err := t.db.Query(`SELECT id, title FROM assignments`)
	if err != nil {
		return nil, fmt.Errorf("listing assignments: %w", err)
	}
	defer rows.Close()

	var assignments []core.Assignment
	for rows.Next() {
		var a core.Assignment
		if err := rows.Scan(&a.ID, &a.Title); err != nil {
			return nil, fmt.Errorf("scanning assignment: %w", err)
		}

		team, err := t.loadAssignmentTeam(a.ID)
		if err != nil {
			return nil, err
		}
		a.Team = team

		feedback, err := t.loadAssignmentFeedback(a.ID)
		if err != nil {
			return nil, err
		}
		a.Feedback = feedback

		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

func (t *TeambackDatabase) LinkIfNecessary(user core.User, externalUser auth.ExternalUserInfo) (auth.Identity, error) {
	identity := auth.Identity{
		UserID:     user.ID,
		ExternalID: externalUser.Sub,
	}

	_, err := t.db.Exec(
		`INSERT INTO identities (user_id, external_id)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id, external_id) DO NOTHING`,
		identity.UserID, identity.ExternalID,
	)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("linking identity: %w", err)
	}
	return identity, nil
}

func (t *TeambackDatabase) loadAssignmentTeam(assignmentID string) ([]core.User, error) {
	rows, err := t.db.Query(
		`SELECT u.id, u.name, u.email, u.role
		 FROM users u
		 JOIN assignment_members am ON am.user_id = u.id
		 WHERE am.assignment_id = $1`, assignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading team: %w", err)
	}
	defer rows.Close()

	var team []core.User
	for rows.Next() {
		var u core.User
		var roleStr string
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &roleStr); err != nil {
			return nil, fmt.Errorf("scanning team member: %w", err)
		}
		u.Role = parseRole(roleStr)
		team = append(team, u)
	}
	return team, rows.Err()
}

func (t *TeambackDatabase) loadAssignmentFeedback(assignmentID string) ([]core.Answer, error) {
	// Load only the latest version of each author's answer
	rows, err := t.db.Query(
		`SELECT a.id, a.author_id, u.name, u.email, u.role, a.version, a.submitted_at
		 FROM answers a
		 JOIN users u ON u.id = a.author_id
		 WHERE a.assignment_id = $1
		   AND a.version = (SELECT MAX(a2.version) FROM answers a2 WHERE a2.assignment_id = a.assignment_id AND a2.author_id = a.author_id)`,
		assignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading feedback: %w", err)
	}
	defer rows.Close()

	var feedback []core.Answer
	for rows.Next() {
		var answerID, authorID, authorName, authorEmail, roleStr string
		var version int
		var submittedAt time.Time
		if err := rows.Scan(&answerID, &authorID, &authorName, &authorEmail, &roleStr, &version, &submittedAt); err != nil {
			return nil, fmt.Errorf("scanning answer: %w", err)
		}

		contribs, err := t.loadContributions(answerID)
		if err != nil {
			return nil, err
		}

		feedback = append(feedback, core.Answer{
			Author: core.User{
				ID:    authorID,
				Name:  authorName,
				Email: authorEmail,
				Role:  parseRole(roleStr),
			},
			MemberContributions: contribs,
			Version:             version,
			SubmittedAt:         submittedAt,
		})
	}
	return feedback, rows.Err()
}

func (t *TeambackDatabase) ListFeedbackHistory(assignmentID string, authorID string) ([]core.Answer, error) {
	rows, err := t.db.Query(
		`SELECT a.id, a.version, a.submitted_at
		 FROM answers a
		 WHERE a.assignment_id = $1 AND a.author_id = $2
		 ORDER BY a.version DESC`, assignmentID, authorID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing feedback history: %w", err)
	}
	defer rows.Close()

	// Load the author info once
	author, err := t.ReadUser(authorID)
	if err != nil {
		return nil, fmt.Errorf("reading author: %w", err)
	}

	var history []core.Answer
	for rows.Next() {
		var answerID string
		var version int
		var submittedAt time.Time
		if err := rows.Scan(&answerID, &version, &submittedAt); err != nil {
			return nil, fmt.Errorf("scanning feedback history: %w", err)
		}

		contribs, err := t.loadContributions(answerID)
		if err != nil {
			return nil, err
		}

		history = append(history, core.Answer{
			Author:              author,
			MemberContributions: contribs,
			Version:             version,
			SubmittedAt:         submittedAt,
		})
	}
	return history, rows.Err()
}

func (t *TeambackDatabase) loadContributions(answerID string) (map[string]core.Contribution, error) {
	rows, err := t.db.Query(
		`SELECT member_id, description, weight FROM contributions WHERE answer_id = $1`, answerID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading contributions: %w", err)
	}
	defer rows.Close()

	contribs := make(map[string]core.Contribution)
	for rows.Next() {
		var memberID string
		var c core.Contribution
		if err := rows.Scan(&memberID, &c.Description, &c.Weight); err != nil {
			return nil, fmt.Errorf("scanning contribution: %w", err)
		}
		contribs[memberID] = c
	}
	return contribs, rows.Err()
}

func parseRole(s string) core.Role {
	if s == "teacher" {
		return core.RoleTeacher
	}
	return core.RoleStudent
}

// SummaryStorage implementation

func (t *TeambackDatabase) CreateSummary(assignmentID string) (core.AssignmentSummary, error) {
	id := uuid.New().String()
	now := time.Now()
	_, err := t.db.Exec(
		`INSERT INTO assignment_summaries (id, assignment_id, status, created_at) VALUES ($1, $2, 'pending', $3)`,
		id, assignmentID, now,
	)
	if err != nil {
		return core.AssignmentSummary{}, fmt.Errorf("creating summary: %w", err)
	}
	return core.AssignmentSummary{
		ID:           id,
		AssignmentID: assignmentID,
		Status:       "pending",
		CreatedAt:    now,
	}, nil
}

func (t *TeambackDatabase) CompleteSummary(id string, summary string, attentionRequired bool) error {
	_, err := t.db.Exec(
		`UPDATE assignment_summaries SET summary = $1, status = 'completed', attention_required = $2, completed_at = NOW() WHERE id = $3`,
		summary, attentionRequired, id,
	)
	if err != nil {
		return fmt.Errorf("completing summary: %w", err)
	}
	return nil
}

func (t *TeambackDatabase) FailSummary(id string) error {
	_, err := t.db.Exec(
		`UPDATE assignment_summaries SET status = 'failed', completed_at = NOW() WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("failing summary: %w", err)
	}
	return nil
}

func (t *TeambackDatabase) GetLatestSummary(assignmentID string) (core.AssignmentSummary, error) {
	var s core.AssignmentSummary
	var completedAt sql.NullTime
	err := t.db.QueryRow(
		`SELECT id, assignment_id, summary, status, attention_required, attempts, created_at, completed_at
		 FROM assignment_summaries
		 WHERE assignment_id = $1
		 ORDER BY created_at DESC LIMIT 1`, assignmentID,
	).Scan(&s.ID, &s.AssignmentID, &s.Summary, &s.Status, &s.AttentionRequired, &s.Attempts, &s.CreatedAt, &completedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.AssignmentSummary{}, fmt.Errorf("no summary found for assignment %s", assignmentID)
		}
		return core.AssignmentSummary{}, fmt.Errorf("getting latest summary: %w", err)
	}
	if completedAt.Valid {
		s.CompletedAt = &completedAt.Time
	}
	return s, nil
}

func (t *TeambackDatabase) ListSummaries(assignmentID string) ([]core.AssignmentSummary, error) {
	rows, err := t.db.Query(
		`SELECT id, assignment_id, summary, status, attention_required, attempts, created_at, completed_at
		 FROM assignment_summaries
		 WHERE assignment_id = $1
		 ORDER BY created_at DESC`, assignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing summaries: %w", err)
	}
	defer rows.Close()

	var summaries []core.AssignmentSummary
	for rows.Next() {
		var s core.AssignmentSummary
		var completedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.AssignmentID, &s.Summary, &s.Status, &s.AttentionRequired, &s.Attempts, &s.CreatedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning summary: %w", err)
		}
		if completedAt.Valid {
			s.CompletedAt = &completedAt.Time
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (t *TeambackDatabase) GetRetryableSummaries(maxAttempts int) ([]core.AssignmentSummary, error) {
	rows, err := t.db.Query(
		`SELECT s.id, s.assignment_id, s.summary, s.status, s.attention_required, s.attempts, s.created_at, s.completed_at
		 FROM assignment_summaries s
		 WHERE s.status = 'failed'
		   AND s.attempts < $1
		   AND NOT EXISTS (
		       SELECT 1 FROM assignment_summaries s2
		       WHERE s2.assignment_id = s.assignment_id AND s2.created_at > s.created_at
		   )`, maxAttempts,
	)
	if err != nil {
		return nil, fmt.Errorf("getting retryable summaries: %w", err)
	}
	defer rows.Close()

	var summaries []core.AssignmentSummary
	for rows.Next() {
		var s core.AssignmentSummary
		var completedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.AssignmentID, &s.Summary, &s.Status, &s.AttentionRequired, &s.Attempts, &s.CreatedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scanning retryable summary: %w", err)
		}
		if completedAt.Valid {
			s.CompletedAt = &completedAt.Time
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (t *TeambackDatabase) ResetForRetry(id string) error {
	_, err := t.db.Exec(
		`UPDATE assignment_summaries SET status = 'pending', attempts = attempts + 1, completed_at = NULL WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("resetting summary for retry: %w", err)
	}
	return nil
}
