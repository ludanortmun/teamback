package handler

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/ludanortmun/teamback/internal/core"
	"github.com/ludanortmun/teamback/internal/middleware"
)

func TestHandleCreateFeedbackRejectsDescriptionOverCharacterLimit(t *testing.T) {
	student := core.User{ID: "student-1", Name: "Ana", Role: core.RoleStudent}
	store := newFeedbackHandlerStorage([]core.User{student}, core.Assignment{
		ID:    "assignment-1",
		Title: "Assignment",
		Team:  []core.User{student},
	})
	h := newFeedbackHandlerForTest(store)

	form := url.Values{
		"description_student-1": {strings.Repeat("a", feedbackDescriptionMaxChars+1)},
		"weight_student-1":      {"100"},
	}
	req := newFeedbackPostRequest("assignment-1", student.ID, form)
	rec := httptest.NewRecorder()

	h.HandleCreateFeedback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no puede superar los 2000 caracteres") {
		t.Fatalf("expected length validation error, got body: %s", rec.Body.String())
	}
	if store.saveAssignmentCalls != 0 || store.saveAnswerVersionCalls != 0 {
		t.Fatal("expected over-limit feedback to be rejected before persisting")
	}
}

func TestHandleCreateFeedbackAcceptsDescriptionAtCharacterLimit(t *testing.T) {
	student := core.User{ID: "student-1", Name: "Ana", Role: core.RoleStudent}
	store := newFeedbackHandlerStorage([]core.User{student}, core.Assignment{
		ID:    "assignment-1",
		Title: "Assignment",
		Team:  []core.User{student},
	})
	h := newFeedbackHandlerForTest(store)

	form := url.Values{
		"description_student-1": {strings.Repeat("á", feedbackDescriptionMaxChars)},
		"weight_student-1":      {"100"},
	}
	req := newFeedbackPostRequest("assignment-1", student.ID, form)
	rec := httptest.NewRecorder()

	h.HandleCreateFeedback(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", rec.Code)
	}
	if store.saveAssignmentCalls != 1 || store.saveAnswerVersionCalls != 1 {
		t.Fatal("expected feedback at character limit to be persisted")
	}
}

func newFeedbackHandlerForTest(store *feedbackHandlerStorage) *Handler {
	tmpl := template.Must(template.New("layout").Parse(`{{define "layout"}}{{range .Errors}}{{.}}{{end}}{{end}}`))
	return &Handler{
		Client:    core.NewClient(store),
		Storage:   store,
		Templates: map[string]*template.Template{"feedback_new.html": tmpl},
	}
}

func newFeedbackPostRequest(assignmentID string, studentID string, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/assignments/"+assignmentID+"/feedback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", assignmentID)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, studentID)
	return req.WithContext(ctx)
}

type feedbackHandlerStorage struct {
	users                  []core.User
	assignments            map[string]core.Assignment
	saveAssignmentCalls    int
	saveAnswerVersionCalls int
}

func newFeedbackHandlerStorage(users []core.User, assignments ...core.Assignment) *feedbackHandlerStorage {
	store := &feedbackHandlerStorage{
		assignments: make(map[string]core.Assignment),
	}
	for _, user := range users {
		store.users = append(store.users, user)
	}
	for _, assignment := range assignments {
		store.assignments[assignment.ID] = assignment
	}
	return store
}

func (s *feedbackHandlerStorage) WriteUser(user core.User) error {
	s.users = append(s.users, user)
	return nil
}

func (s *feedbackHandlerStorage) ReadUser(id string) (core.User, error) {
	for _, user := range s.users {
		if user.ID == id {
			return user, nil
		}
	}
	return core.User{}, errors.New("user not found")
}

func (s *feedbackHandlerStorage) ReadUserByEmail(email string) (core.User, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return core.User{}, errors.New("user not found")
}

func (s *feedbackHandlerStorage) SaveAssignment(assignment core.Assignment) error {
	s.saveAssignmentCalls++
	s.assignments[assignment.ID] = assignment
	return nil
}

func (s *feedbackHandlerStorage) GetAssignment(assignmentID string) (core.Assignment, error) {
	assignment, ok := s.assignments[assignmentID]
	if !ok {
		return core.Assignment{}, errors.New(core.ErrAssignmentNotFound)
	}
	return assignment, nil
}

func (s *feedbackHandlerStorage) ListAssignments() ([]core.Assignment, error) {
	assignments := make([]core.Assignment, 0, len(s.assignments))
	for _, assignment := range s.assignments {
		assignments = append(assignments, assignment)
	}
	return assignments, nil
}

func (s *feedbackHandlerStorage) ListFeedbackHistory(assignmentID string, authorID string) ([]core.Answer, error) {
	assignment, ok := s.assignments[assignmentID]
	if !ok {
		return nil, errors.New(core.ErrAssignmentNotFound)
	}
	var history []core.Answer
	for _, answer := range assignment.Feedback {
		if answer.Author.ID == authorID {
			history = append(history, answer)
		}
	}
	return history, nil
}

func (s *feedbackHandlerStorage) SaveAnswerVersion(assignmentID string, answer core.Answer) error {
	s.saveAnswerVersionCalls++
	assignment, ok := s.assignments[assignmentID]
	if !ok {
		return errors.New(core.ErrAssignmentNotFound)
	}
	if index := slices.IndexFunc(assignment.Feedback, func(existing core.Answer) bool {
		return existing.Author.ID == answer.Author.ID
	}); index >= 0 {
		assignment.Feedback[index] = answer
	} else {
		assignment.Feedback = append(assignment.Feedback, answer)
	}
	s.assignments[assignmentID] = assignment
	return nil
}
