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

	"github.com/ludanortmun/teamback/internal/auth"
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
	req := newFeedbackPostRequest("assignment-1", student.ID, form, h.Sessions)
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
	req := newFeedbackPostRequest("assignment-1", student.ID, form, h.Sessions)
	rec := httptest.NewRecorder()

	h.HandleCreateFeedback(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/assignments/assignment-1" {
		t.Fatalf("expected redirect to assignment success page, got %q", location)
	}
	if store.saveAssignmentCalls != 1 || store.saveAnswerVersionCalls != 1 {
		t.Fatal("expected feedback at character limit to be persisted")
	}
}

func TestHandleViewAssignmentShowsFeedbackSuccessMessage(t *testing.T) {
	student := core.User{ID: "student-1", Name: "Ana", Role: core.RoleStudent}
	store := newFeedbackHandlerStorage([]core.User{student}, core.Assignment{
		ID:    "assignment-1",
		Title: "Assignment",
		Team:  []core.User{student},
	})
	h := newAssignmentHandlerForTest(store)

	flashReq := httptest.NewRequest(http.MethodGet, "/assignments/assignment-1", nil)
	addSessionCookie(flashReq, h.Sessions, student.ID)
	flashRec := httptest.NewRecorder()
	if err := h.Sessions.SetFlash(flashRec, flashReq, "Tu retroalimentación se envió correctamente. Puedes revisarla abajo."); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/assignments/assignment-1", nil)
	req.SetPathValue("id", "assignment-1")
	for _, cookie := range flashRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, student.ID)
	rec := httptest.NewRecorder()

	h.HandleViewAssignment(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Tu retroalimentación se envió correctamente") {
		t.Fatalf("expected success message, got body: %s", rec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/assignments/assignment-1", nil)
	secondReq.SetPathValue("id", "assignment-1")
	for _, cookie := range rec.Result().Cookies() {
		secondReq.AddCookie(cookie)
	}
	secondRec := httptest.NewRecorder()

	h.HandleViewAssignment(secondRec, secondReq.WithContext(ctx))

	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", secondRec.Code)
	}
	if strings.Contains(secondRec.Body.String(), "Tu retroalimentación se envió correctamente") {
		t.Fatalf("expected success message to be cleared, got body: %s", secondRec.Body.String())
	}
}

func newFeedbackHandlerForTest(store *feedbackHandlerStorage) *Handler {
	tmpl := template.Must(template.New("layout").Parse(`{{define "layout"}}{{range .Errors}}{{.}}{{end}}{{end}}`))
	return &Handler{
		Client:    core.NewClient(store),
		Sessions:  newTestSessionStore(),
		Storage:   store,
		Templates: map[string]*template.Template{"feedback_new.html": tmpl},
	}
}

func newAssignmentHandlerForTest(store *feedbackHandlerStorage) *Handler {
	tmpl := template.Must(template.New("layout").Parse(`{{define "layout"}}{{range .Successes}}{{.}}{{end}}{{end}}`))
	return &Handler{
		Client:    core.NewClient(store),
		Sessions:  newTestSessionStore(),
		Storage:   store,
		Templates: map[string]*template.Template{"assignment.html": tmpl},
	}
}

func newFeedbackPostRequest(assignmentID string, studentID string, form url.Values, sessions *auth.SessionStore) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/assignments/"+assignmentID+"/feedback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", assignmentID)
	addSessionCookie(req, sessions, studentID)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, studentID)
	return req.WithContext(ctx)
}

func newTestSessionStore() *auth.SessionStore {
	return auth.NewSessionStore("0123456789abcdef0123456789abcdef")
}

func addSessionCookie(req *http.Request, sessions *auth.SessionStore, userID string) {
	rec := httptest.NewRecorder()
	sessions.Set(rec, userID)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
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

func (s *feedbackHandlerStorage) ListStudents() ([]core.User, error) {
	var students []core.User
	for _, user := range s.users {
		if user.Role == core.RoleStudent {
			students = append(students, user)
		}
	}
	return students, nil
}

func (s *feedbackHandlerStorage) DeleteUser(id string) error {
	for i, user := range s.users {
		if user.ID == id && user.Role == core.RoleStudent {
			s.users = append(s.users[:i], s.users[i+1:]...)
			return nil
		}
	}
	return errors.New("user not found or not a student")
}

func (s *feedbackHandlerStorage) SaveAssignment(assignment core.Assignment) error {
	s.saveAssignmentCalls++
	s.assignments[assignment.ID] = assignment
	return nil
}

func (s *feedbackHandlerStorage) DeleteAssignment(id string) error {
	if _, ok := s.assignments[id]; !ok {
		return errors.New(core.ErrAssignmentNotFound)
	}
	delete(s.assignments, id)
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
