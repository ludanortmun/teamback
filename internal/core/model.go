package core

type Role int

const (
	RoleStudent Role = iota
	RoleTeacher
)

// User represents the profile information for a user of the system.
type User struct {
	ID    string
	Name  string
	Email string
	Role  Role
}

// Assignment represents a team assignment.
// It includes the team members, assignment title, and submitted feedback answers.
type Assignment struct {
	ID       string
	Team     []User
	Title    string
	Feedback []Answer
}

// Answer represents a student's feedback and self-reflection.
// To be valid, it must include the student's own contribution description and weight,
// as well as descriptions and weights for each teammate's contribution. Weights must sum 100%.
type Answer struct {
	Author              User
	MemberContributions map[string]Contribution
}

// Contribution contains a plain-text description of a student's contribution to the Assignment,
// as well as a contribution weight, representing the percentage of work done by said student.
type Contribution struct {
	Description string
	Weight      uint8
}
