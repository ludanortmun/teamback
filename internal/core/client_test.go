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
		ID:    "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name:  "Tom John",
		Email: "tom.john@school.edu",
		Role:  RoleTeacher,
	}
	student1 = User{
		ID:    "4fcc9f90-627b-418d-ab59-bf34c7a5b9e4",
		Name:  "John Doe",
		Email: "john.doe@school.edu",
		Role:  RoleStudent,
	}
	student2 = User{
		ID:    "48483e93-4c63-4f96-875b-abdada724bba",
		Name:  "Jane Doe",
		Email: "jane.doe@school.edu",
		Role:  RoleStudent,
	}
	student3 = User{
		ID:    "fb402398-a4c0-4ba9-92ac-99ca2708adbe",
		Name:  "Jack Doe",
		Email: "jack.doe@school.edu",
		Role:  RoleStudent,
	}
	// nonTeacher is a student used to test unauthorized teacher-only actions.
	nonTeacher = User{
		ID:    "a6b2bbf8-ef1e-459b-87ec-24375cc3c90d",
		Name:  "Tom John",
		Email: "tom.john@school.edu",
		Role:  RoleStudent,
	}
)

// callerCtx returns a context carrying the given user's ID as the caller key.
func callerCtx(user User) context.Context {
	return context.WithValue(context.Background(), CtxCallerKey, user.ID)
}

func TestClient_AddUser_MissingCaller(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)

	_, err := client.AddUser(context.Background(), student1)

	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_AddUser_Unauthorized(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, nonTeacher)

	_, err := client.AddUser(callerCtx(nonTeacher), student1)

	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_AddUser_Success(t *testing.T) {
	stg := newFakeStorage()
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
	client := NewClient(newFakeStorage())

	_, err := client.CreateAssignment(context.Background(), "Some assignment", []User{})

	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_CreateAssignment_Unauthorized(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, nonTeacher)

	_, err := client.CreateAssignment(callerCtx(nonTeacher), "Some assignment", []User{})

	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_CreateAssignment_Success(t *testing.T) {
	stg := newFakeStorage()
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

func TestClient_DeleteAssignment_Unauthorized(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, nonTeacher)

	err := client.DeleteAssignment(callerCtx(nonTeacher), "assignment-1")
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_DeleteAssignment_Success(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1)
	stg.assignments["assignment-1"] = Assignment{ID: "assignment-1", Title: "Some assignment", Team: []User{student1}}

	if err := client.DeleteAssignment(callerCtx(teacher), "assignment-1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := stg.assignments["assignment-1"]; ok {
		t.Fatal("expected assignment to be deleted")
	}
}

func TestClient_AddFeedback_AssignmentNotFound(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, student1)

	_, err := client.AddFeedback(callerCtx(student1), "invalid id", Answer{})
	if err == nil || err.Error() != ErrAssignmentNotFound {
		t.Fatal("expected error with message", ErrAssignmentNotFound)
	}
}

func TestClient_AddFeedback_Unauthorized(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2, student3)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Some assignment", []User{student1, student2})

	// student3 is not a member of the assignment team
	_, err := client.AddFeedback(callerCtx(student3), assignment.ID, Answer{})
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_AddFeedback_MissingUser(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Some assignment", []User{student1, student2})

	_, err := client.AddFeedback(callerCtx(student3), assignment.ID, Answer{})
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_AddFeedback_MissingContribution(t *testing.T) {
	stg := newFakeStorage()
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
	stg := newFakeStorage()
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
	stg := newFakeStorage()
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
	stg := newFakeStorage()
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
	stg := newFakeStorage()
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

func TestClient_AddFeedback_OverwriteExisting(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	initial := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Initial self review", Weight: 50},
			student2.ID: {Description: "Initial peer review", Weight: 50},
		},
	}
	updated := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Updated self review", Weight: 40},
			student2.ID: {Description: "Updated peer review", Weight: 60},
		},
	}

	if _, err := client.AddFeedback(callerCtx(student1), assignment.ID, initial); err != nil {
		t.Fatal(err)
	}
	result, err := client.AddFeedback(callerCtx(student1), assignment.ID, updated)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Feedback) != 1 {
		t.Fatalf("expected one feedback entry after overwrite, got %d", len(result.Feedback))
	}
	if result.Feedback[0].Author.ID != student1.ID {
		t.Fatal("expected feedback author to remain student1")
	}
	if result.Feedback[0].MemberContributions[student1.ID].Description != "Updated self review" {
		t.Fatal("expected updated feedback data to be returned")
	}
	if result.Feedback[0].MemberContributions[student2.ID].Weight != 60 {
		t.Fatal("expected updated feedback weights to be returned")
	}

	stored := stg.assignments[assignment.ID]
	if len(stored.Feedback) != 1 {
		t.Fatalf("expected one persisted feedback entry after overwrite, got %d", len(stored.Feedback))
	}
	if stored.Feedback[0].MemberContributions[student1.ID].Description != "Updated self review" {
		t.Fatal("expected persisted feedback to be overwritten")
	}
}

func TestClient_AddFeedback_OverwriteDoesNotAffectOthers(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	student1Initial := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Student1 initial self review", Weight: 50},
			student2.ID: {Description: "Student1 initial peer review", Weight: 50},
		},
	}
	student2Answer := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Student2 peer review", Weight: 45},
			student2.ID: {Description: "Student2 self review", Weight: 55},
		},
	}
	student1Updated := Answer{
		MemberContributions: map[string]Contribution{
			student1.ID: {Description: "Student1 updated self review", Weight: 35},
			student2.ID: {Description: "Student1 updated peer review", Weight: 65},
		},
	}

	if _, err := client.AddFeedback(callerCtx(student1), assignment.ID, student1Initial); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddFeedback(callerCtx(student2), assignment.ID, student2Answer); err != nil {
		t.Fatal(err)
	}
	result, err := client.AddFeedback(callerCtx(student1), assignment.ID, student1Updated)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Feedback) != 2 {
		t.Fatalf("expected two feedback entries after overwrite, got %d", len(result.Feedback))
	}

	student1FeedbackIndex := slices.IndexFunc(result.Feedback, func(answer Answer) bool {
		return answer.Author.ID == student1.ID
	})
	if student1FeedbackIndex < 0 {
		t.Fatal("expected student1 feedback to exist")
	}
	if result.Feedback[student1FeedbackIndex].MemberContributions[student1.ID].Description != "Student1 updated self review" {
		t.Fatal("expected student1 feedback to be updated")
	}
	if result.Feedback[student1FeedbackIndex].MemberContributions[student2.ID].Weight != 65 {
		t.Fatal("expected student1 feedback weights to be updated")
	}

	student2FeedbackIndex := slices.IndexFunc(result.Feedback, func(answer Answer) bool {
		return answer.Author.ID == student2.ID
	})
	if student2FeedbackIndex < 0 {
		t.Fatal("expected student2 feedback to exist")
	}
	if result.Feedback[student2FeedbackIndex].MemberContributions[student2.ID].Description != "Student2 self review" {
		t.Fatal("expected student2 feedback to remain unchanged")
	}
	if result.Feedback[student2FeedbackIndex].MemberContributions[student1.ID].Weight != 45 {
		t.Fatal("expected student2 feedback weights to remain unchanged")
	}
}

func TestClient_GetAssignment_NotFound(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher)

	_, err := client.GetAssignment(callerCtx(teacher), "nonexistent")
	if err == nil || err.Error() != ErrAssignmentNotFound {
		t.Fatal("expected error with message", ErrAssignmentNotFound)
	}
}

func TestClient_GetAssignment_TeacherSeesAll(t *testing.T) {
	stg := newFakeStorage()
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
	stg := newFakeStorage()
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
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2, student3)

	assignment, _ := client.CreateAssignment(callerCtx(teacher), "Assignment", []User{student1, student2})

	_, err := client.GetAssignment(callerCtx(student3), assignment.ID)
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected error with message", ErrUnauthorizedMsg)
	}
}

func TestClient_GetAssignments_MissingCaller(t *testing.T) {
	client := NewClient(newFakeStorage())

	_, err := client.GetAssignments(context.Background())
	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_GetAssignments_TeacherSeesAll(t *testing.T) {
	stg := newFakeStorage()
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
	stg := newFakeStorage()
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
	stg := newFakeStorage()
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

func TestClient_GetUserByEmail_MissingCaller(t *testing.T) {
	client := NewClient(newFakeStorage())

	_, err := client.GetUserByEmail(context.Background(), "john.doe@school.edu")

	if err == nil || err.Error() != ErrMissingCallerKey {
		t.Fatal("expected error with message", ErrMissingCallerKey)
	}
}

func TestClient_GetUserByEmail_NotFound(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher)

	_, err := client.GetUserByEmail(callerCtx(teacher), "nonexistent@school.edu")

	if err == nil || err.Error() != ErrUserNotFound {
		t.Fatal("expected error with message", ErrUserNotFound)
	}
}

func TestClient_GetUserByEmail_Success(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1)

	result, err := client.GetUserByEmail(callerCtx(teacher), student1.Email)
	if err != nil {
		t.Fatal(err)
	}

	if result.ID != student1.ID {
		t.Fatal("expected user ID to match")
	}
	if result.Email != student1.Email {
		t.Fatal("expected user email to match")
	}
	if result.Name != student1.Name {
		t.Fatal("expected user name to match")
	}
}

func TestClient_ListStudents_TeacherOnly(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student2, student1)

	students, err := client.ListStudents(callerCtx(teacher))
	if err != nil {
		t.Fatal(err)
	}
	if len(students) != 2 {
		t.Fatalf("expected 2 students, got %d", len(students))
	}
	if students[0].Role != RoleStudent || students[1].Role != RoleStudent {
		t.Fatal("expected only students to be returned")
	}

	_, err = client.ListStudents(callerCtx(student1))
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected unauthorized error for student")
	}
}

func TestClient_DeleteUser_TeacherOnly(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	if err := client.DeleteUser(callerCtx(teacher), student1.ID); err != nil {
		t.Fatal(err)
	}
	if len(stg.users) != 2 {
		t.Fatalf("expected 2 users after delete, got %d", len(stg.users))
	}
	if slices.ContainsFunc(stg.users, func(u User) bool { return u.ID == student1.ID }) {
		t.Fatal("expected deleted student to be removed")
	}

	if err := client.DeleteUser(callerCtx(student2), teacher.ID); err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected unauthorized error for student")
	}
}

func TestClient_GetAssignmentSummary_StudentUnauthorized(t *testing.T) {
	stg := newFakeStorage()
	ss := &fakeSummaryStorage{}
	client := NewClient(stg, WithSummarizer(nil, ss, nil))
	stg.users = append(stg.users, student1)

	_, err := client.GetAssignmentSummary(callerCtx(student1), "assignment-1")
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected unauthorized error for student")
	}
}

func TestClient_TriggerSummarySync_Disabled(t *testing.T) {
	client := NewClient(newFakeStorage())

	_, err := client.TriggerSummarySync("assignment-1")
	if err == nil || err.Error() != "AI features are disabled" {
		t.Fatal("expected AI disabled error")
	}
}

func TestClient_TriggerSummarySync_Success(t *testing.T) {
	stg := newFakeStorage()
	ss := &fakeSummaryStorage{}
	client := NewClient(stg, WithSummarizer(fakeSummarizer{}, ss, nil))

	assignment := Assignment{
		ID:    "assignment-1",
		Title: "Assignment 1",
		Team:  []User{student1, student2},
		Feedback: []Answer{
			{Author: student1},
		},
	}
	stg.assignments[assignment.ID] = assignment

	summary, err := client.TriggerSummarySync(assignment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "completed" {
		t.Fatalf("expected completed summary, got %s", summary.Status)
	}
	if summary.Summary != "generated summary" {
		t.Fatalf("expected generated summary, got %q", summary.Summary)
	}
	if !summary.AttentionRequired {
		t.Fatal("expected attention required to be true")
	}
}

func TestClient_ListFeedbackHistory_StudentUnauthorized(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, student1)

	_, err := client.ListFeedbackHistory(callerCtx(student1), "assignment-1", "author-1")
	if err == nil || err.Error() != ErrUnauthorizedMsg {
		t.Fatal("expected unauthorized error for student")
	}
}

func TestClient_ListFeedbackHistory_TeacherSuccess(t *testing.T) {
	stg := newFakeStorage()
	client := NewClient(stg)
	stg.users = append(stg.users, teacher, student1, student2)

	assignment := Assignment{
		ID:   "a1",
		Team: []User{student1, student2},
		Feedback: []Answer{
			{Author: student1, MemberContributions: map[string]Contribution{
				student1.ID: {Description: "Did work", Weight: 50},
				student2.ID: {Description: "Also worked", Weight: 50},
			}},
		},
	}
	stg.assignments["a1"] = assignment

	history, err := client.ListFeedbackHistory(callerCtx(teacher), "a1", student1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
}

type fakeSummarizer struct{}

func (fakeSummarizer) Summarize(assignment Assignment, feedback []Answer) (SummaryResult, error) {
	return SummaryResult{Summary: "generated summary", AttentionRequired: true}, nil
}

type fakeSummaryStorage struct {
	summaries []AssignmentSummary
}

func (f *fakeSummaryStorage) CreateSummary(assignmentID string) (AssignmentSummary, error) {
	s := AssignmentSummary{ID: "sum-1", AssignmentID: assignmentID, Status: "pending", Attempts: 1}
	f.summaries = append(f.summaries, s)
	return s, nil
}

func (f *fakeSummaryStorage) CompleteSummary(id string, summary string, attentionRequired bool) error {
	for i := range f.summaries {
		if f.summaries[i].ID == id {
			f.summaries[i].Status = "completed"
			f.summaries[i].Summary = summary
			f.summaries[i].AttentionRequired = attentionRequired
		}
	}
	return nil
}

func (f *fakeSummaryStorage) FailSummary(id string) error {
	for i := range f.summaries {
		if f.summaries[i].ID == id {
			f.summaries[i].Status = "failed"
		}
	}
	return nil
}

func (f *fakeSummaryStorage) GetLatestSummary(assignmentID string) (AssignmentSummary, error) {
	for i := len(f.summaries) - 1; i >= 0; i-- {
		if f.summaries[i].AssignmentID == assignmentID {
			return f.summaries[i], nil
		}
	}
	return AssignmentSummary{}, errors.New("no summary found")
}

func (f *fakeSummaryStorage) GetLatestSummaries(assignmentIDs []string) ([]AssignmentSummary, error) {
	var result []AssignmentSummary
	for _, assignmentID := range assignmentIDs {
		for i := len(f.summaries) - 1; i >= 0; i-- {
			if f.summaries[i].AssignmentID == assignmentID {
				result = append(result, f.summaries[i])
				break
			}
		}
	}
	return result, nil
}

func (f *fakeSummaryStorage) ListSummaries(assignmentID string) ([]AssignmentSummary, error) {
	var result []AssignmentSummary
	for _, s := range f.summaries {
		if s.AssignmentID == assignmentID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (f *fakeSummaryStorage) GetRetryableSummaries(maxAttempts int) ([]AssignmentSummary, error) {
	var result []AssignmentSummary
	for _, s := range f.summaries {
		if s.Status == "failed" && s.Attempts < maxAttempts {
			result = append(result, s)
		}
	}
	return result, nil
}

func (f *fakeSummaryStorage) ResetForRetry(id string) error {
	for i := range f.summaries {
		if f.summaries[i].ID == id {
			f.summaries[i].Status = "pending"
			f.summaries[i].Attempts++
		}
	}
	return nil
}

type fakeStorage struct {
	users       []User
	assignments map[string]Assignment
}

func newFakeStorage() *fakeStorage {
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

func (f *fakeStorage) ReadUserByEmail(email string) (User, error) {
	for _, user := range f.users {
		if user.Email == email {
			return user, nil
		}
	}
	return User{}, errors.New("user not found")
}

func (f *fakeStorage) ListStudents() ([]User, error) {
	var students []User
	for _, user := range f.users {
		if user.Role == RoleStudent {
			students = append(students, user)
		}
	}
	return students, nil
}

func (f *fakeStorage) DeleteUser(id string) error {
	for i, user := range f.users {
		if user.ID == id && user.Role == RoleStudent {
			f.users = append(f.users[:i], f.users[i+1:]...)
			return nil
		}
	}
	return errors.New("user not found or not a student")
}

func (f *fakeStorage) SaveAssignment(assignment Assignment) error {
	f.assignments[assignment.ID] = assignment
	return nil
}

func (f *fakeStorage) DeleteAssignment(id string) error {
	if _, ok := f.assignments[id]; !ok {
		return errors.New(ErrAssignmentNotFound)
	}
	delete(f.assignments, id)
	return nil
}

func (f *fakeStorage) SaveAnswerVersion(assignmentID string, answer Answer) error {
	assignment, ok := f.assignments[assignmentID]
	if !ok {
		return errors.New(ErrAssignmentNotFound)
	}
	if index := slices.IndexFunc(assignment.Feedback, func(existing Answer) bool {
		return existing.Author.ID == answer.Author.ID
	}); index >= 0 {
		assignment.Feedback[index] = answer
	} else {
		assignment.Feedback = append(assignment.Feedback, answer)
	}
	f.assignments[assignmentID] = assignment
	return nil
}

func (f *fakeStorage) GetAssignment(assignmentId string) (Assignment, error) {
	assignment, ok := f.assignments[assignmentId]
	if !ok {
		return Assignment{}, errors.New(ErrAssignmentNotFound)
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

func (f *fakeStorage) ListFeedbackHistory(assignmentID string, authorID string) ([]Answer, error) {
	assignment, ok := f.assignments[assignmentID]
	if !ok {
		return nil, errors.New(ErrAssignmentNotFound)
	}
	var history []Answer
	for _, a := range assignment.Feedback {
		if a.Author.ID == authorID {
			history = append(history, a)
		}
	}
	return history, nil
}
