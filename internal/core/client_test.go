package core

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// Shared test users
var (
	teacher = User{
		ID:   "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name: "Tom John",
		Role: RoleTeacher,
	}
	student1 = User{
		ID:   "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name: "John Doe",
		Role: RoleStudent,
	}
	student2 = User{
		ID:   "48483e93-4c63-4f96-875b-abdada724bba",
		Name: "Jane Doe",
		Role: RoleStudent,
	}
	student3 = User{
		ID:   "fb402398-a4c0-4ba9-92ac-99ca2708adbe",
		Name: "Jack Doe",
		Role: RoleStudent,
	}
	// nonTeacher is a student used to test unauthorized teacher-only actions.
	nonTeacher = User{
		ID:   "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name: "Tom John",
		Role: RoleStudent,
	}
)

// callerCtx returns a context carrying the given user's ID as the caller key.
func callerCtx(user User) context.Context {
	return context.WithValue(context.Background(), CtxCallerKey, user.ID)
}

func TestClient_AddUser_MissingCaller(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)

	_, err := client.AddUser(context.Background(), student1)

	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_AddUser_Unauthorized(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, nonTeacher)

	_, err := client.AddUser(callerCtx(nonTeacher), student1)

	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_AddUser_Success(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher)

	result, err := client.AddUser(callerCtx(teacher), User{Name: "John Doe", Role: RoleStudent})
	if err != nil {
		t.Fatal(err)
	}

	if result.ID == "" {
		t.Fatal("expected user ID to be initialized")
	}
	if result.Name != "John Doe" {
		t.Fatal("expected user name to be preserved")
	}

	found := false
	for _, u := range stg.users {
		if u.ID == result.ID && u.Name == result.Name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected user to be added to storage, but it was not")
	}
}

func TestClient_CreateAssignment_MissingCaller(t *testing.T) {
	client := NewClient(newFakeStorge())

	_, err := client.CreateAssignment(context.Background(), "Some assignment", []User{})

	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_CreateAssignment_Unauthorized(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, nonTeacher)

	_, err := client.CreateAssignment(callerCtx(nonTeacher), "Some assignment", []User{})

	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_CreateAssignment_Success(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, err := client.CreateAssignment(callerCtx(teacher), "Some assignment", []User{student1, student2})
	if err != nil {
		t.Fatal(err)
	}

	if assignment.Title != "Some assignment" {
		t.Fatal("expected assignment title")
	}
	if !(slices.Contains(assignment.Team, student1) && slices.Contains(assignment.Team, student2)) {
		t.Fatal("expected assignment team")
	}
	if len(assignment.Feedback) > 0 {
		t.Fatal("expected assignment feedback to be empty")
	}
}

func TestClient_AddFeedback_AssignmentNotFound(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, student1)

	_, err := client.AddFeedback(callerCtx(student1), "invalid id", Answer{})
	if err == nil || err.Error() != ErrAssignmentNotFound {
		t.Fatal("expected error with message", ErrAssignmentNotFound)
	}
}

func TestClient_AddFeedback_Unauthorized(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Some assignment", []User{student1, student2})

	// student3 is not a member of the assignment team
	_, err := client.AddFeedback(callerCtx(student3), assignment.ID, Answer{})
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_AddFeedback_MissingContribution(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	// Only include contribution for student1, missing student2
	answer := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Did stuff", Weight: 100},
		},
	}
	_, err := client.AddFeedback(callerCtx(student1), assignment.ID, answer)
	if err == nil || err.Error() != ErrInvalidContributions {
		t.Fatal("expected error with message", ErrInvalidContributions)
	}
}

func TestClient_AddFeedback_ExtraContribution(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	// Include an extra contribution for student3 who is not in the team
	answer := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Did stuff", Weight: 50},
			student2.ID: {Description: "Did stuff", Weight: 25},
			student3.ID: {Description: "Extra", Weight: 25},
		},
	}
	_, err := client.AddFeedback(callerCtx(student1), assignment.ID, answer)
	if err == nil || err.Error() != ErrInvalidContributions {
		t.Fatal("expected error with message", ErrInvalidContributions)
	}
}

func TestClient_AddFeedback_EmptyDescription(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	answer := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Did stuff", Weight: 50},
			student2.ID: {Description: "", Weight: 50},
		},
	}
	_, err := client.AddFeedback(callerCtx(student1), assignment.ID, answer)
	if err == nil || err.Error() != ErrEmptyDescription {
		t.Fatal("expected error with message", ErrEmptyDescription)
	}
}

func TestClient_AddFeedback_WeightsNotSumTo100(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	answer := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Did stuff", Weight: 60},
			student2.ID: {Description: "Did stuff", Weight: 60},
		},
	}
	_, err := client.AddFeedback(callerCtx(student1), assignment.ID, answer)
	if err == nil || err.Error() != ErrInvalidWeights {
		t.Fatal("expected error with message", ErrInvalidWeights)
	}
}

func TestClient_AddFeedback_Success(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	answer := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "I did the backend", Weight: 50},
			student2.ID: {Description: "She did the frontend", Weight: 50},
		},
	}
	result, err := client.AddFeedback(callerCtx(student1), assignment.ID, answer)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Feedback) != 1 {
		t.Fatal("expected one feedback entry")
	}
	if result.Feedback[0].Author.ID != student1.ID {
		t.Fatal("expected feedback author to be set to caller")
	}

	// Verify persistence
	stored := stg.assignments[assignment.ID]
	if len(stored.Feedback) != 1 {
		t.Fatal("expected feedback to be persisted")
	}
}

func TestClient_GetAssignment_NotFound(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher)

	_, err := client.GetAssignment(callerCtx(teacher), "nonexistent")
	if err == nil || err.Error() != ErrAssignmentNotFound {
		t.Fatal("expected error with message", ErrAssignmentNotFound)
	}
}

func TestClient_GetAssignment_TeacherSeesAll(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	answer1 := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "My work", Weight: 50},
			student2.ID: {Description: "Their work", Weight: 50},
		},
	}
	answer2 := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Their work", Weight: 40},
			student2.ID: {Description: "My work", Weight: 60},
		},
	}
	client.AddFeedback(callerCtx(student1), assignment.ID, answer1)
	client.AddFeedback(callerCtx(student2), assignment.ID, answer2)

	result, err := client.GetAssignment(callerCtx(teacher), assignment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Feedback) != 2 {
		t.Fatal("expected teacher to see all 2 answers, got", len(result.Feedback))
	}
}

func TestClient_GetAssignment_StudentSeesOwnOnly(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	answer1 := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "My work", Weight: 50},
			student2.ID: {Description: "Their work", Weight: 50},
		},
	}
	answer2 := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Their work", Weight: 40},
			student2.ID: {Description: "My work", Weight: 60},
		},
	}
	client.AddFeedback(callerCtx(student1), assignment.ID, answer1)
	client.AddFeedback(callerCtx(student2), assignment.ID, answer2)

	result, err := client.GetAssignment(callerCtx(student1), assignment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Feedback) != 1 {
		t.Fatal("expected student to see only their own answer, got", len(result.Feedback))
	}
	if result.Feedback[0].Author.ID != student1.ID {
		t.Fatal("expected the visible answer to be authored by the caller")
	}
}

func TestClient_GetAssignment_UnauthorizedStudent(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2, student3)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	_, err := client.GetAssignment(callerCtx(student3), assignment.ID)
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_GetAssignments_MissingCaller(t *testing.T) {
	client := NewClient(newFakeStorge())

	_, err := client.GetAssignments(context.Background())
	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_GetAssignments_TeacherSeesAll(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2, student3)

	client.CreateAssignment(callerCtx(teacher), "Assignment 1", []User{student1, student2})
	client.CreateAssignment(callerCtx(teacher), "Assignment 2", []User{student2, student3})

	results, err := client.GetAssignments(callerCtx(teacher))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatal("expected teacher to see all 2 assignments, got", len(results))
	}
}

func TestClient_GetAssignments_StudentSeesOwnOnly(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2, student3)

	a1, _ := client.CreateAssignment(callerCtx(teacher), "Assignment 1", []User{student1, student2})
	client.CreateAssignment(callerCtx(teacher), "Assignment 2", []User{student2, student3})

	// student1 submits feedback on assignment 1
	answer := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "My work", Weight: 50},
			student2.ID: {Description: "Their work", Weight: 50},
		},
	}
	client.AddFeedback(callerCtx(student1), a1.ID, answer)

	results, err := client.GetAssignments(callerCtx(student1))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatal("expected student1 to see only 1 assignment, got", len(results))
	}
	if results[0].Title != "Assignment 1" {
		t.Fatal("expected student1 to see Assignment 1")
	}
	if len(results[0].Feedback) != 1 {
		t.Fatal("expected student to see only their own feedback")
	}
	if results[0].Feedback[0].Author.ID != student1.ID {
		t.Fatal("expected feedback to be authored by student1")
	}
}

func TestClient_GetAssignments_StudentNoAssignments(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2, student3)

	client.CreateAssignment(callerCtx(teacher), "Assignment 1", []User{student1, student2})

	results, err := client.GetAssignments(callerCtx(student3))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatal("expected student3 to see no assignments, got", len(results))
	}
}

type fakeStorage struct {
	users       []User
	assignments map[string]Assignment
}

func newFakeStorge() *fakeStorage {
	return &fakeStorage{
		users:       make([]User, 0),
		assignments: make(map[string]Assignment),
	}
}

func (f *fakeStorage) WriteUser(user User) error {
	f.users = append(f.users, user)
	return nil
}

func (f *fakeStorage) ReadUser(userId string) (User, error) {
	for _, user := range f.users {
		if user.ID == userId {
			return user, nil
		}
	}
	return User{}, errors.New("user not found")
}

func (f *fakeStorage) SaveAssignment(assignment Assignment) error {
	f.assignments[assignment.ID] = assignment
	return nil
}

func (f *fakeStorage) GetAssignment(assignmentId string) (Assignment, error) {
	assignment, ok := f.assignments[assignmentId]
	if !ok {
		return Assignment{}, errors.New("assignment not found")
	}
	return assignment, nil
}

func (f *fakeStorage) ListAssignments() ([]Assignment, error) {
	result := make([]Assignment, 0, len(f.assignments))
	for _, a := range f.assignments {
		result = append(result, a)
	}
	return result, nil
}
