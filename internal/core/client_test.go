package core

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestClient_AddUser_MissingCaller(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	ctx := context.Background()

	newUser := User{
		ID:   "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name: "John Doe",
		Role: RoleStudent,
	}

	err := client.AddUser(ctx, newUser)

	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_AddUser_Unauthorized(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)

	teacherUser := User{
		ID:   "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name: "Tom John",
		Role: RoleStudent,
	}
	stg.users = append(stg.users, teacherUser)
	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxCallerKey, teacherUser.ID)

	newUser := User{
		ID:   "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name: "John Doe",
		Role: RoleStudent,
	}

	err := client.AddUser(ctx, newUser)

	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_AddUser_Success(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)

	teacherUser := User{
		ID:   "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name: "Tom John",
		Role: RoleTeacher,
	}
	stg.users = append(stg.users, teacherUser)
	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxCallerKey, teacherUser.ID)

	newUser := User{
		ID:   "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name: "John Doe",
		Role: RoleStudent,
	}

	_ = client.AddUser(ctx, newUser)

	if !slices.Contains(stg.users, newUser) {
		t.Errorf("expected user to be added to storage, but it was not")
	}
}

func TestClient_CreateAssignment_MissingCaller(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)
	ctx := context.Background()

	_, err := client.CreateAssignment(ctx, "Some assignment", []User{})

	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_CreateAssignment_Unauthorized(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)

	studentCaller := User{
		ID:   "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name: "Tom John",
		Role: RoleStudent,
	}
	stg.users = append(stg.users, studentCaller)
	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxCallerKey, studentCaller.ID)

	_, err := client.CreateAssignment(ctx, "Some assignment", []User{})

	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_CreateAssignment_Success(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)

	teacherUser := User{
		ID:   "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name: "Tom John",
		Role: RoleTeacher,
	}

	student1 := User{
		ID:   "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name: "John Doe",
		Role: RoleStudent,
	}

	student2 := User{
		ID:   "48483e93-4c63-4f96-875b-abdada724bba",
		Name: "Jane Doe",
		Role: RoleStudent,
	}
	stg.users = append(stg.users, teacherUser, student1, student2)
	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxCallerKey, teacherUser.ID)

	assignment, err := client.CreateAssignment(ctx, "Some assignment", []User{student1, student2})
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

	student1 := User{
		ID:   "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name: "John Doe",
		Role: RoleStudent,
	}
	stg.users = append(stg.users, student1)
	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxCallerKey, student1.ID)
	_, err := client.AddFeedback(ctx, "invalid id", Answer{})
	if err == nil || err.Error() != ErrAssignmentNotFound {
		t.Fatal("expected error with message", ErrAssignmentNotFound)
	}
}

func TestClient_AddFeedback_Unauthorized(t *testing.T) {
	stg := newFakeStorge()
	client := NewClient(stg)

	teacherUser := User{
		ID:   "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name: "Tom John",
		Role: RoleTeacher,
	}

	student1 := User{
		ID:   "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name: "John Doe",
		Role: RoleStudent,
	}

	student2 := User{
		ID:   "48483e93-4c63-4f96-875b-abdada724bba",
		Name: "Jane Doe",
		Role: RoleStudent,
	}
	stg.users = append(stg.users, teacherUser, student1, student2)
	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxCallerKey, teacherUser.ID)

	assignment, _ := client.CreateAssignment(ctx, "Some assignment", []User{student1, student2})

	student3 := User{
		ID:   "fb402398-a4c0-4ba9-92ac-99ca2708adbe",
		Name: "Jack Doe",
		Role: RoleStudent,
	}
	ctx = context.WithValue(ctx, CtxCallerKey, student3.ID)
	_, err := client.AddFeedback(ctx, assignment.ID, Answer{})
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
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
